package parkour

import (
	"time"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// sessionState ...
type sessionState int

const (
	stateIdle sessionState = iota
	stateCountdown
	stateRunning
)

// Session tracks one player's parkour run.
//
// All Session fields are mutated only on the world owner. Delayed countdown
// work is scheduled with player.DoAfter so dragonfly per-player state is only
// ever read/written on the owner that owns it.
type Session struct {
	handle *world.EntityHandle

	courseID string
	state    sessionState

	startPos          mgl64.Vec3
	endPos            mgl64.Vec3
	completionRadius  float64
	checkpoint        mgl64.Vec3
	runStart          time.Time
	checkpointElapsed time.Duration
	rankName          string

	countdownTask       *world.Task
	countdownGeneration uint64
}

// session ...
func (m *Manager) session(p *player.Player) *Session {
	s, exists := m.sessions.Load(p.UUID().String())
	if !exists {
		return nil
	}
	sess, exists := s.(*Session)
	if !exists {
		return nil
	}
	return sess
}

// ensureSession returns the existing session for p, or creates a new one
// bound to p's EntityHandle. The handle is captured once at creation so
// delayed countdown tasks can re-enter without holding a stale *player.Player.
func (m *Manager) ensureSession(p *player.Player) *Session {
	if sess := m.session(p); sess != nil {
		return sess
	}
	sess := &Session{handle: p.H()}
	m.sessions.Store(p.UUID().String(), sess)
	return sess
}

// stopCountdown cancels an in-flight countdown task. Safe to call from the
// world owner.
func (s *Session) stopCountdown() {
	s.countdownGeneration++
	if task := s.countdownTask; task != nil {
		s.countdownTask = nil
		task.Cancel()
	}
}

// beginCountdown starts an N-second countdown that fires tick once per
// second (counting down from seconds to 1) and then fires onDone.
//
// Both callbacks run on the world owner via player.Do / player.DoAfter.
// If the player has left by the time a callback runs, the task fails and no
// callback fires.
func (s *Session) beginCountdown(seconds int, tick func(*player.Player, int), onDone func(*world.Tx, *player.Player)) {
	s.stopCountdown()
	generation := s.countdownGeneration

	handle := s.handle
	if handle == nil {
		return
	}

	var scheduleNext func(remaining int)
	scheduleNext = func(remaining int) {
		s.countdownTask = player.DoAfter(handle, time.Second, func(tx *world.Tx, p *player.Player) {
			if generation != s.countdownGeneration {
				return
			}
			if remaining > 1 {
				if tick != nil {
					tick(p, remaining-1)
				}
				scheduleNext(remaining - 1)
				return
			}
			s.countdownTask = nil
			if onDone != nil {
				onDone(tx, p)
			}
		})
	}

	if seconds > 0 {
		player.Do(handle, func(_ *world.Tx, p *player.Player) {
			if generation != s.countdownGeneration {
				return
			}
			if tick != nil {
				tick(p, seconds)
			}
		})
		scheduleNext(seconds)
		return
	}

	player.Do(handle, func(tx *world.Tx, p *player.Player) {
		if generation != s.countdownGeneration {
			return
		}
		if onDone != nil {
			onDone(tx, p)
		}
	})
}
