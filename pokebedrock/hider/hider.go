// Package hider manages per-player visibility toggles, letting players hide or
// show other players in the hub.
package hider

import (
	"slices"
	"sync"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/parkour"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/slapper"
)

// Manager ...
type Manager struct {
	mu     sync.RWMutex
	hidden map[string]struct{}
}

var global *Manager

// NewManager ...
func NewManager() *Manager {
	m := &Manager{
		hidden: make(map[string]struct{}),
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
//
// HandleJoin runs on the joining player's world owner, so it must operate on
// p.Tx() directly. Never call world.Call / Task.Wait from here — that deadlocks
// the owner on itself.
func (m *Manager) HandleJoin(p *player.Player) {
	hidden := m.snapshotHidden()
	if len(hidden) == 0 {
		return
	}

	for ent := range p.Tx().Players() {
		other := ent.(*player.Player)
		if other.H() == p.H() {
			continue
		}
		if _, ok := hidden[other.UUID().String()]; ok {
			other.HideEntity(p)
		}
	}
}

// HandleQuit ...
func (m *Manager) HandleQuit(p *player.Player) {
	m.setHidden(p, false)
}

// hideAll ...
//
// Runs on p's world owner (Toggle is called from a packet handler), so it uses
// p.Tx() directly instead of scheduling a nested owner wait.
func (m *Manager) hideAll(p *player.Player) {
	exempted := m.exemptedPlayers()
	for ent := range p.Tx().Players() {
		other := ent.(*player.Player)
		if other.H() == p.H() || slices.Contains(exempted, ent.H()) {
			continue
		}
		p.HideEntity(other)
	}
}

// showAll ...
//
// Runs on p's world owner (Toggle is called from a packet handler), so it uses
// p.Tx() directly instead of scheduling a nested owner wait.
func (m *Manager) showAll(p *player.Player) {
	for ent := range p.Tx().Players() {
		other := ent.(*player.Player)
		if other.H() == p.H() {
			continue
		}
		p.ShowEntity(other)
	}
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
	for _, s := range slapper.All() {
		if h := s.Handle(); h != nil {
			set[h] = struct{}{}
		}
	}

	if manager := parkour.Global(); manager != nil {
		for _, h := range manager.NPCHandles() {
			if h != nil {
				set[h] = struct{}{}
			}
		}
	}

	handles := make([]*world.EntityHandle, 0, len(set))
	for h := range set {
		handles = append(handles, h)
	}
	return handles
}
