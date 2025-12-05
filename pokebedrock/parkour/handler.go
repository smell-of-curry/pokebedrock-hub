package parkour

import (
	"github.com/df-mc/dragonfly/server/player"
	"github.com/go-gl/mathgl/mgl64"
)

// HandleMove ...
func (m *Manager) HandleMove(p *player.Player, pos mgl64.Vec3) {
	sess := m.session(p)

	thresholdY := m.w.Spawn().Vec3().Y() - 30
	if pos.Y() < thresholdY {
		if sess != nil && (sess.state == stateRunning || sess.state == stateCountdown) {
			m.restartFromCheckpoint(p, sess)
			return
		}
		p.Teleport(m.w.Spawn().Vec3Middle())
		return
	}

	if sess == nil || sess.state != stateRunning {
		return
	}

	m.checkCheckpoint(p, pos, sess)
	m.checkFinish(p, pos, sess)
}

// HandleQuit ...
func (m *Manager) HandleQuit(p *player.Player) {
	sess := m.session(p)
	if sess == nil {
		return
	}
	sess.stopCountdown()
	m.sessions.Delete(p.UUID().String())
}

// HandleItemUse ...
func (m *Manager) HandleItemUse(p *player.Player, action string) {
	sess := m.session(p)
	if sess == nil {
		return
	}
	switch action {
	case "quit":
		m.endRun(p, sess, true)
	case "restart":
		m.restartFromCheckpoint(p, sess)
	}
}
