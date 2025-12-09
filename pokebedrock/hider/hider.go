package hider

import (
	"slices"
	"sync"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/parkour"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/slapper"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

// Manager ...
type Manager struct {
	mu     sync.RWMutex
	hidden map[string]struct{}
	w      *world.World
}

var global *Manager

// NewManager ...
func NewManager(w *world.World) *Manager {
	m := &Manager{
		hidden: make(map[string]struct{}),
		w:      w,
	}
	global = m
	return m
}

// Global ...
func Global() *Manager {
	return global
}

// Toggle ...
func (m *Manager) Toggle(p *player.Player) {
	if m.hasHidden(p) {
		m.showAll(p)
		m.setHidden(p, false)
		p.SendJukeboxPopup(text.Colourf("<green>Players shown.</green>"))
		return
	}

	m.hideAll(p)
	m.setHidden(p, true)
	p.SendJukeboxPopup(text.Colourf("<yellow>Players hidden.</yellow>"))
}

// HandleJoin ...
func (m *Manager) HandleJoin(p *player.Player) {
	hidden := m.snapshotHidden()
	if len(hidden) == 0 {
		return
	}

	m.w.Exec(func(tx *world.Tx) {
		for ent := range tx.Players() {
			other := ent.(*player.Player)
			if other == p {
				continue
			}
			if _, ok := hidden[other.UUID().String()]; ok {
				other.HideEntity(p)
			}
		}
	})
}

// HandleQuit ...
func (m *Manager) HandleQuit(p *player.Player) {
	m.setHidden(p, false)
}

// hideAll ...
func (m *Manager) hideAll(p *player.Player) {
	exempted := m.exemptedPlayers()
	m.w.Exec(func(tx *world.Tx) {
		for ent := range tx.Players() {
			other := ent.(*player.Player)
			if other == p || slices.Contains(exempted, ent.H()) {
				continue
			}
			p.HideEntity(other)
		}
	})
}

// showAll ...
func (m *Manager) showAll(p *player.Player) {
	m.w.Exec(func(tx *world.Tx) {
		for ent := range tx.Players() {
			other := ent.(*player.Player)
			if other == p {
				continue
			}
			p.ShowEntity(other)
		}
	})
}

// hasHidden ...
func (m *Manager) hasHidden(p *player.Player) bool {
	m.mu.RLock()
	_, ok := m.hidden[p.UUID().String()]
	m.mu.RUnlock()
	return ok
}

// setHidden ...
func (m *Manager) setHidden(p *player.Player, hidden bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if hidden {
		m.hidden[p.UUID().String()] = struct{}{}
		return
	}
	delete(m.hidden, p.UUID().String())
}

// snapshotHidden ...
func (m *Manager) snapshotHidden() map[string]struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]struct{}, len(m.hidden))
	for k, v := range m.hidden {
		out[k] = v
	}
	return out
}

// exemptedPlayers ...
func (m *Manager) exemptedPlayers() []*world.EntityHandle {
	set := make(map[*world.EntityHandle]struct{})
	for _, s := range srv.All() {
		id := s.Identifier()
		if h := slapper.FromIdentifier(id).Handle(); h != nil {
			set[h] = struct{}{}
		}
	}

	for _, h := range parkour.Global().NPCHandles() {
		if h != nil {
			set[h] = struct{}{}
		}
	}

	handles := make([]*world.EntityHandle, 0, len(set))
	for h := range set {
		handles = append(handles, h)
	}
	return handles
}
