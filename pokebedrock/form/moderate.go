// Package form provides forms for the server.
package form

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/form"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/moderation"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/session"
)

// Moderate represents a form for moderating a specific player by their name.
// It displays options for creating or removing inflictions.
type Moderate struct {
	target string
}

// NewModerate creates a new moderation menu for the specified target player.
// It returns a form.Menu with buttons for different moderation options.
func NewModerate(target string) form.Menu {
	f := form.NewMenu(Moderate{target: target}, text.Colourf("<yellow>Moderating '%s'</yellow>", target))
	btns := []form.Button{
		form.NewButton("Create an Infliction", ""),
		form.NewButton("Remove an Infliction", ""),
	}

	return f.WithButtons(btns...)
}

// Submit handles the submission of the moderation form.
// It processes the selected button and redirects to the appropriate action form.
func (m Moderate) Submit(sub form.Submitter, b form.Button, _ *world.Tx) {
	p := sub.(*player.Player)

	switch strings.ToLower(text.Clean(b.Text)) {
	case "create an infliction":
		p.SendForm(NewCreateInfliction(m.target))
	case "remove an infliction":
		p.Messagef("%s", text.Colourf("<green>Processing inflictions for %s...</green>", m.target))
		h := p.H()

		go func() {
			f := NewRemoveInfliction(m.target)

			h.ExecWorld(func(_ *world.Tx, e world.Entity) {
				p = e.(*player.Player)
				p.SendForm(f)
			})
		}()
	}
}

// CreateInfliction represents a form for creating a new infliction on a player.
// It contains fields for the type of infliction, expiry time, and reason.
type CreateInfliction struct {
	InflictionType form.Dropdown
	Expiry         form.Input
	Reason         form.Input

	target string
}

// NewCreateInfliction creates a new form for adding an infliction to the specified target player.
// It initializes the form with dropdown options for infliction types and default input values.
func NewCreateInfliction(target string) form.Custom {
	inflictionTypes := []string{
		string(moderation.InflictionBanned),
		string(moderation.InflictionMuted),
		string(moderation.InflictionFrozen),
		string(moderation.InflictionWarned),
		string(moderation.InflictionKicked),
	}

	f := form.New(CreateInfliction{
		InflictionType: form.NewDropdown("Infliction Type:", inflictionTypes, 0),
		Expiry:         form.NewInput("Expiry (in minutes, blank = forever):", "", "30"),
		Reason:         form.NewInput("Reason", "", "Guy was being bad"),

		target: target,
	}, text.Colourf("<yellow>Creating infliction on '%s'</yellow>", target))

	return f
}

// Submit handles the submission of the create infliction form.
// It processes the form data, creates the infliction, and applies it to the target player if they're online.
func (c CreateInfliction) Submit(sub form.Submitter, _ *world.Tx) {
	prosecutor := sub.(*player.Player)

	inf := c.InflictionType.Options[c.InflictionType.Value()]
	infType := moderation.InflictionType(inf)

	var permanent bool

	var expiry int64

	if c.Expiry.Value() == "" {
		permanent = true
	}

	if !permanent {
		exp, err := strconv.Atoi(c.Expiry.Value())
		if err != nil {
			prosecutor.Message(text.Colourf("<red>Invalid expiry value provided.</red>"))

			return
		}

		expiry = time.Now().UnixMilli() + time.Minute.Milliseconds()*int64(exp)
	}

	reason := c.Reason.Value()
	if reason == "" {
		reason = "None provided"
	}

	infliction := moderation.Infliction{
		Type:          infType,
		DateInflicted: time.Now().UnixMilli(),
		Reason:        reason,
		Prosecutor:    prosecutor.Name(),
	}
	if !permanent {
		infliction.ExpiryDate = &expiry
	}

	h := prosecutor.H()
	go func() {
		h.ExecWorld(func(tx *world.Tx, e world.Entity) {
			prosecutor = e.(*player.Player)
			if prosecutor == nil {
				return
			}

			err := moderation.GlobalService().AddInfliction(moderation.ModelRequest{
				Name:             c.target,
				InflictionStatus: moderation.InflictionStatusCurrent,
				Infliction:       infliction,
			})
			if err != nil {
				prosecutor.Message(text.Colourf("<red>Error while adding infliction on '%s' %s.</red>", c.target, err.Error()))

				return
			}

			prosecutor.Message(text.Colourf("<green>Added infliction on '%s'.</green>", c.target))

			for ent := range tx.Players() {
				victim := ent.(*player.Player)
				if !strings.EqualFold(victim.Name(), c.target) {
					continue
				}

				handler, ok := victim.Handler().(inflictionHandler)
				if !ok {
					continue
				}

				switch infliction.Type {
				case moderation.InflictionMuted:
					exp := infliction.ExpiryDate
					if exp != nil && *exp != 0 {
						handler.Inflictions().SetMuteDuration(*exp)
					}

					handler.Inflictions().SetMuted(true)
				case moderation.InflictionFrozen:
					handler.Inflictions().SetFrozen(true)
					victim.SetImmobile()
				case moderation.InflictionWarned:
					victim.Message(text.Colourf("<yellow>You've been warned for %s.</yellow>", infliction.Reason))
				case moderation.InflictionKicked:
					victim.Disconnect(text.Colourf("<red>You've been kicked."))
				case moderation.InflictionBanned:
					victim.Disconnect(text.Colourf("<red>You've been banned! Reason: %s, Expiry Date: %d, Prosecutor: %s</red>",
						infliction.Reason, infliction.ExpiryDate, infliction.Prosecutor))
				}
			}
		})
	}()
}

// RemoveInfliction represents a form for removing existing inflictions from a player.
// It contains the target player name and a map of inflictions to their display labels.
type RemoveInfliction struct {
	target        string
	inflictionMap map[string]moderation.Infliction
}

// NewRemoveInfliction creates a new form for removing inflictions from the specified target player.
// It fetches current inflictions and displays them as buttons.
func NewRemoveInfliction(target string) form.Menu {
	resp, err := moderation.GlobalService().InflictionOfName(target)
	if err != nil {
		return form.NewMenu(RemoveInfliction{target: target}, text.Colourf("<yellow>Moderating '%s'</yellow>", target)).
			WithButtons(form.NewButton("Error fetching inflictions", ""))
	}

	var btns []form.Button

	inflictionMap := make(map[string]moderation.Infliction)

	if len(resp.CurrentInflictions) == 0 {
		btns = append(btns, form.NewButton("No inflictions found", ""))
	} else {
		for _, inf := range resp.CurrentInflictions {
			name := fmt.Sprintf("[%s] - Reason: %s", string(inf.Type), inf.Reason)
			description := fmt.Sprintf("By: %s, Till: %d", inf.Prosecutor, inf.ExpiryDate)
			label := fmt.Sprintf("%s\n%s", name, description)
			inflictionMap[label] = inf

			btns = append(btns, form.NewButton(fmt.Sprintf("%s\n%s", name, description), ""))
		}
	}

	f := form.NewMenu(RemoveInfliction{
		target: target, inflictionMap: inflictionMap,
	}, text.Colourf("<yellow>Moderating '%s'</yellow>", target))

	return f.WithButtons(btns...)
}

// Submit handles the submission of the remove infliction form.
// It processes the selected infliction and removes it from the target player.
func (r RemoveInfliction) Submit(sub form.Submitter, b form.Button, _ *world.Tx) {
	prosecutor := sub.(*player.Player)

	if b.Text == "No inflictions found" {
		return
	}

	infliction, ok := r.inflictionMap[b.Text]
	if !ok {
		prosecutor.Message(text.Colourf("<red>Infliction not found within map.</red>"))

		return
	}

	h := prosecutor.H()
	go func() {
		h.ExecWorld(func(tx *world.Tx, e world.Entity) {
			prosecutor = e.(*player.Player)

			err := moderation.GlobalService().RemoveInfliction(moderation.ModelRequest{
				Name:             r.target,
				InflictionStatus: moderation.InflictionStatusCurrent,
				Infliction:       infliction,
			})
			if err != nil {
				prosecutor.Message(text.Colourf("<red>Error while removing infliction on '%s'.</red>", r.target))

				return
			}

			prosecutor.Message(text.Colourf("<green>Removed infliction on '%s'.</green>", r.target))

			for ent := range tx.Players() {
				victim := ent.(*player.Player)
				if !strings.EqualFold(victim.Name(), r.target) {
					continue
				}

				handler, ok := victim.Handler().(inflictionHandler)
				if !ok {
					continue
				}

				switch infliction.Type {
				case moderation.InflictionMuted:
					handler.Inflictions().SetMuted(false)
				case moderation.InflictionFrozen:
					handler.Inflictions().SetFrozen(false)
					victim.SetMobile()
				}
			}
		})
	}()
}

// inflictionHandler defines the interface for handlers that can manage player inflictions.
// It requires an Inflictions method that returns the player's infliction state container.
type inflictionHandler interface {
	Inflictions() *session.Inflictions
}
