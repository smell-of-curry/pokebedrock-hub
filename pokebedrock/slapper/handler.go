package slapper

import (
	"github.com/df-mc/dragonfly/server/player"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/form"
)

// handleInteract ...
func (s *Slapper) handleInteract(p *player.Player) {
	p.SendForm(form.NewServerConfirm(s.Server()))
}
