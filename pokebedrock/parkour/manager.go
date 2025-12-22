package parkour

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/npc"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/block"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/kit"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/slapper"
)

// Manager ...
type Manager struct {
	courses  map[string]CourseConfig
	sessions sync.Map

	cfg Config
	lb  *leaderboard
	w   *world.World
	log *slog.Logger

	leaderboardTexts map[string]*world.EntityHandle
	leaderboardMu    sync.RWMutex

	npcHandles map[string]*world.EntityHandle
	npcMu      sync.RWMutex
}

var globalManager *Manager

// Global ...
func Global() *Manager {
	return globalManager
}

// NewManager ...
func NewManager(log *slog.Logger, w *world.World, cfg Config) *Manager {
	m := &Manager{
		courses: make(map[string]CourseConfig),

		cfg: cfg,
		lb:  newLeaderboard(cfg.LeaderboardPath),
		w:   w,
		log: log,

		leaderboardTexts: make(map[string]*world.EntityHandle),
		npcHandles:       make(map[string]*world.EntityHandle),
	}
	for _, course := range cfg.Courses {
		m.courses[course.Identifier] = course
	}
	m.spawnNPCs()
	m.spawnLeaderboardTexts()

	globalManager = m
	return m
}

// spawnNPCs ...
func (m *Manager) spawnNPCs() {
	for _, course := range m.courses {
		sk := slapper.FromIdentifier(course.Identifier)
		m.w.Exec(func(tx *world.Tx) {
			n := npc.Create(npc.Settings{
				Name:     text.Colourf("<green>%s</green>", course.Name),
				Skin:     sk.Skin(),
				Position: course.NPC.Position.vec3(),
				Yaw:      course.NPC.Yaw,
				Pitch:    course.NPC.Pitch,
				Scale:    course.NPC.Scale,
				Immobile: true,
			}, tx, func(p *player.Player) {
				m.startForm(p, course)
			})

			m.npcMu.Lock()
			m.npcHandles[course.Identifier] = n.H()
			m.npcMu.Unlock()
		})
	}
}

// spawnLeaderboardTexts ...
func (m *Manager) spawnLeaderboardTexts() {
	for _, course := range m.courses {
		m.updateLeaderboardText(course.Identifier)
	}
}

// StartCourse ...
func (m *Manager) StartCourse(p *player.Player, courseID string, rankName string) error {
	course, ok := m.courses[courseID]
	if !ok {
		return fmt.Errorf("unknown course %s", courseID)
	}

	sess := m.ensureSession(p)
	if sess.state == stateRunning || sess.state == stateCountdown {
		p.Message(text.Colourf("<red>You're already in a parkour run.</red>"))
		return nil
	}

	sess.courseID = course.Identifier
	sess.state = stateCountdown
	sess.checkpoint = course.Start.vec3().Add(mgl64.Vec3{0.5, 0, 0.5})
	sess.rankName = rankName
	sess.startPos = course.Start.vec3()
	sess.endPos = course.End.vec3()
	sess.completionRadius = m.cfg.CompletionRadius

	kit.Apply(kit.Parkour, p)

	p.Teleport(sess.startPos)
	p.SetImmobile()

	sess.beginCountdown(m.cfg.CountdownSeconds, func(remaining int) {
		p.SendJukeboxPopup(text.Colourf("<yellow>Starting in %d...</yellow>", remaining))
	}, func() {
		p.H().ExecWorld(func(_ *world.Tx, e world.Entity) {
			p, exists := e.(*player.Player)
			if !exists {
				return
			}
			sess.state = stateRunning
			sess.runStart = time.Now()
			p.SetMobile()
			p.SendJukeboxPopup(text.Colourf("<green>Go!</green>"))
		})
	})
	return nil
}

// Restart ...
func (m *Manager) restartFromCheckpoint(p *player.Player, sess *Session) {
	if sess.state == stateIdle || sess.state == stateCountdown {
		return
	}

	sess.stopCountdown()
	sess.state = stateCountdown

	p.Teleport(sess.checkpoint)
	p.SetImmobile()

	sess.beginCountdown(m.cfg.CountdownSeconds, func(remaining int) {
		p.SendJukeboxPopup(text.Colourf("<yellow>Restarting in %d...</yellow>", remaining))
	}, func() {
		p.H().ExecWorld(func(_ *world.Tx, e world.Entity) {
			p, exists := e.(*player.Player)
			if !exists {
				return
			}
			sess.state = stateRunning
			sess.runStart = time.Now()
			p.SetMobile()
			p.SendJukeboxPopup(text.Colourf("<green>Go!</green>"))
		})
	})
}

// checkCheckpoint ...
func (m *Manager) checkCheckpoint(p *player.Player, pos mgl64.Vec3, sess *Session) {
	if sess.state != stateRunning {
		return
	}
	tx := p.Tx()
	blockPos := cube.PosFromVec3(pos)
	checkpoint := blockPos.Vec3().Add(mgl64.Vec3{0.5, 0, 0.5})
	if _, ok := tx.Block(blockPos).(block.PressurePlate); ok && !sess.checkpoint.ApproxEqual(checkpoint) {
		sess.checkpoint = checkpoint
		sess.checkpointElapsed = time.Since(sess.runStart) + sess.checkpointElapsed
		sess.runStart = time.Now()
		p.SendJukeboxPopup(text.Colourf("<green>Checkpoint saved.</green>"))
	}
}

// checkFinish ...
func (m *Manager) checkFinish(p *player.Player, pos mgl64.Vec3, sess *Session) {
	if sess.state != stateRunning {
		return
	}

	if pos.Sub(sess.endPos).Len() > sess.completionRadius {
		return
	}

	sess.state = stateIdle
	duration := time.Since(sess.runStart) + sess.checkpointElapsed
	m.handleFinish(p, sess, duration)
}

// handleFinish ...
func (m *Manager) handleFinish(p *player.Player, sess *Session, dur time.Duration) {
	course, exists := m.courses[sess.courseID]
	if !exists {
		return
	}

	bestPlacement, prevBest, err := m.lb.update(sess.courseID, p.XUID(), p.Name(), sess.rankName, dur)
	if err != nil {
		m.log.Error("parkour leaderboard update failed", "error", err)
	}

	if bestPlacement > 0 && bestPlacement <= 10 {
		p.Message(text.Colourf(
			"<green>Congratulations! You reached rank %d on %s with %0.2fs.</green>",
			bestPlacement, course.Name, dur.Seconds(),
		))
	}

	var message string
	if prevBest > 0 {
		diff := dur - prevBest
		switch {
		case diff < 0:
			message = text.Colourf(
				"<yellow>You finished %0.2fs faster than your last run! (previous: %0.2fs, current: %0.2fs)</yellow>",
				(-diff).Seconds(), prevBest.Seconds(), dur.Seconds(),
			)
		case diff > 0:
			message = text.Colourf(
				"<red>You were %0.2fs slower than your last run. (previous: %0.2fs, current: %0.2fs)</red>",
				diff.Seconds(), prevBest.Seconds(), dur.Seconds(),
			)
		default:
			message = text.Colourf(
				"<gray>You matched your previous time exactly at %0.2fs.</gray>",
				dur.Seconds(),
			)
		}
	} else {
		message = text.Colourf(
			"<green>Your time of %0.2fs has been recorded as your personal best!</green>",
			dur.Seconds(),
		)
	}
	p.Message(message)

	m.updateLeaderboardText(sess.courseID)
	m.endRun(p, sess, false)
}

// endRun ...
func (m *Manager) endRun(p *player.Player, sess *Session, teleportSpawn bool) {
	sess.stopCountdown()
	m.sessions.Delete(p.UUID().String())
	if teleportSpawn {
		p.Teleport(p.Tx().World().Spawn().Vec3Middle())
	}
	p.SetMobile()
	kit.Apply(kit.Lobby, p)
}
