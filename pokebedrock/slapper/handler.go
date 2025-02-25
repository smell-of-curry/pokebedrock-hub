package slapper

import (
	"github.com/df-mc/dragonfly/server/player"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/form"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// handleInteract ...
func (s *Slapper) handleInteract(p *player.Player) {
	h, ok := p.Handler().(rankHandler)
	if !ok {
		return
	}

	if p.Sneaking() && h.Rank() >= rank.Admin {
		// TODO: Admin form
		return
	}

	p.SendForm(form.NewServerConfirm(s.Server()))
}

// rankHandler ...
type rankHandler interface {
	Rank() rank.Rank
}
