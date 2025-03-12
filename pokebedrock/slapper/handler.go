package slapper

import (
	"github.com/df-mc/dragonfly/server/player"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/form"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/session"
)

// handleInteract ...
func (s *Slapper) handleInteract(p *player.Player) {
	_, ok := p.Handler().(rankHandler)
	if !ok {
		return
	}

	p.SendForm(form.NewServerConfirm(s.Server()))
}

// rankHandler ...
type rankHandler interface {
	Ranks() *session.Ranks
}
