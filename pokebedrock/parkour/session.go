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
// All Session fields are mutated only on the world transaction goroutine.
// The countdown goroutine never touches the player directly; it routes
// callbacks through handle.ExecWorld so dragonfly's per-player state is
// only ever read/written on the Tx goroutine that owns it.
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

	countdownStop chan struct{}
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
// the countdown goroutine can dispatch via ExecWorld without holding a
// stale *player.Player.
func (m *Manager) ensureSession(p *player.Player) *Session {
	if sess := m.session(p); sess != nil {
		return sess
	}
	sess := &Session{handle: p.H()}
	m.sessions.Store(p.UUID().String(), sess)
	return sess
}

// stopCountdown signals an in-flight countdown goroutine to exit. Safe to
// call from the Tx goroutine; the countdown goroutine itself never
// touches countdownStop.
func (s *Session) stopCountdown() {
	if channel := s.countdownStop; channel != nil {
		s.countdownStop = nil
		close(channel)
	}
}

// beginCountdown starts an N-second countdown that fires tick once per
// second (counting down from seconds to 1) and then fires onDone.
//
// Both callbacks are invoked on the world transaction goroutine via
// s.handle.ExecWorld, so they receive a valid *player.Player and may
// safely call methods on it. If the player has left the world by the
// time a callback runs, ExecWorld drops it and no callback fires. This
// is the only correct way to schedule per-tick player work from a
// background goroutine — calling *player.Player methods directly from
// the countdown goroutine has been observed to corrupt dragonfly's
// per-session slices and crash the runtime inside gcWriteBarrier.
func (s *Session) beginCountdown(seconds int, tick func(*player.Player, int), onDone func(*world.Tx, *player.Player)) {
	s.stopCountdown()

	handle := s.handle
	if handle == nil {
		return
	}

	s.countdownStop = make(chan struct{})

	go func(stop <-chan struct{}, sec int) {
		for i := sec; i > 0; i-- {
			select {
			case <-stop:
				return
			default:
			}
			if tick != nil {
				remaining := i
				handle.ExecWorld(func(_ *world.Tx, e world.Entity) {
					if p, ok := e.(*player.Player); ok {
						tick(p, remaining)
					}
				})
			}
			time.Sleep(time.Second)
		}
		select {
		case <-stop:
			return
		default:
			if onDone != nil {
				handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
					if p, ok := e.(*player.Player); ok {
						onDone(tx, p)
					}
				})
			}
		}
	}(s.countdownStop, seconds)
}
