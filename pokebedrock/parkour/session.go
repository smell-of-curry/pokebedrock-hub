package parkour

import (
	"time"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/go-gl/mathgl/mgl64"
)

// sessionState ...
type sessionState int

const (
	stateIdle sessionState = iota
	stateCountdown
	stateRunning
)

// Session ...
type Session struct {
	courseID string
	state    sessionState

	startPos         mgl64.Vec3
	endPos           mgl64.Vec3
	completionRadius float64
	checkpoint       mgl64.Vec3
	runStart         time.Time
	rankName         string

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

// ensureSession ...
func (m *Manager) ensureSession(p *player.Player) *Session {
	if sess := m.session(p); sess != nil {
		return sess
	}
	sess := &Session{}
	m.sessions.Store(p.UUID().String(), sess)
	return sess
}

// stopCountdown ...
func (s *Session) stopCountdown() {
	if s.countdownStop == nil {
		return
	}
	close(s.countdownStop)
	s.countdownStop = nil
}

// beginCountdown ...
func (s *Session) beginCountdown(seconds int, tick func(int), onDone func()) {
	s.stopCountdown()
	s.countdownStop = make(chan struct{})

	go func(stop <-chan struct{}, sec int) {
		for i := sec; i > 0; i-- {
			select {
			case <-stop:
				return
			default:
			}
			if tick != nil {
				tick(i)
			}
			time.Sleep(time.Second)
		}
		select {
		case <-stop:
			return
		default:
			onDone()
		}
	}(s.countdownStop, seconds)
}
