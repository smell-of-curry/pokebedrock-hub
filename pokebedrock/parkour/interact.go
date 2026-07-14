package parkour

import (
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
)

// InteractPlayer opens the parkour start form when the player targets one of the
// course NPC entities.
func (m *Manager) InteractPlayer(p *player.Player, target world.Entity) bool {
	targetH := target.H()

	m.npcMu.RLock()
	defer m.npcMu.RUnlock()

	for id, handle := range m.npcHandles {
		if handle != targetH {
			continue
		}
		course, ok := m.courses[id]
		if !ok {
			return false
		}
		m.startForm(p, course)
		return true
	}

	return false
}
