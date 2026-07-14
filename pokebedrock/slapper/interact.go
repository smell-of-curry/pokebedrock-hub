package slapper

import (
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
)

// InteractPlayer opens this server's transfer form when the player targets one of
// the hub slapper entities.
func InteractPlayer(p *player.Player, target world.Entity) bool {
	targetH := target.H()

	for _, s := range All() {
		if s.Handle() != targetH {
			continue
		}
		s.handleInteract(p)
		s.SendAnimation(p)
		return true
	}

	return false
}
