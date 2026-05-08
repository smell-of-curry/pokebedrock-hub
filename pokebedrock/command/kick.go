// Package command provides commands for the server.
package command

import (
	"log/slog"
	"time"

	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/moderation"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// Kick represents a command to kick a player from the server.
// It takes a target player and an optional reason parameter.
type Kick struct {
	Target []cmd.Target `name:"target"`
	Reason string       `name:"reason" optional:"true" type:"text"`

	rankAllower
}

// NewKick creates a new kick command with the specified rank requirement.
func NewKick(r rank.Rank) cmd.Command {
	return cmd.New("kick", "Kick a player from the server", []string{"k"}, Kick{rankAllower: rankAllower{rank: r}})
}

// Run executes the kick command.
func (k Kick) Run(src cmd.Source, o *cmd.Output, _ *world.Tx) {
	p := src.(*player.Player)

	reason := k.Reason
	if reason == "" {
		reason = "No reason provided"
	}

	prosecutorName := p.Name()

	for _, target := range k.Target {
		victim := target.(*player.Player)
		victimName := victim.Name()

		// Create the kick infliction
		infliction := moderation.Infliction{
			Type:          moderation.InflictionKicked,
			DateInflicted: time.Now().UnixMilli(),
			Reason:        reason,
			Prosecutor:    prosecutorName,
		}

		// Sync to the moderation service off the world transaction goroutine.
		go func(name string) {
			if err := moderation.GlobalService().AddInfliction(moderation.ModelRequest{
				Name:             name,
				InflictionStatus: moderation.InflictionStatusCurrent,
				Infliction:       infliction,
			}); err != nil {
				slog.Default().Error("error while syncing kick globally", "target", name, "error", err)
			}
		}(victimName)

		// Kick the player
		victim.Disconnect(text.Colourf("<red>You've been kicked. Reason: %s</red>", reason))
		o.Print("Successfully kicked from world", "target", victimName, "reason", reason)
	}
}
