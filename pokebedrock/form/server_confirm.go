package form

import (
	"fmt"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/form"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/queue"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/session"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

// ServerConfirm ...
type ServerConfirm struct {
	srv *srv.Server

	YesButton form.Button
	NoButton  form.Button
}

// NewServerConfirm ...
func NewServerConfirm(srv *srv.Server) form.Modal {
	f := form.NewModal(ServerConfirm{srv, form.YesButton(), form.NoButton()},
		text.Colourf("<purple>Server Navigator</purple>")).
		WithBody(fmt.Sprintf("Are you sure you want to join %s?", srv.Name()))
	return f
}

// Submit ...
func (f ServerConfirm) Submit(sub form.Submitter, b form.Button, _ *world.Tx) {
	if f.srv == nil || b != f.YesButton {
		return
	}

	p := sub.(*player.Player)
	h, ok := p.Handler().(rankHandler)
	if !ok {
		return
	}

	queue.QueueManager.AddPlayer(p, h.Ranks().HighestRank(), f.srv)
}

// rankHandler ...
type rankHandler interface {
	Ranks() *session.Ranks
}
