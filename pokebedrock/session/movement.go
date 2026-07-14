package session

import (
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

// Movement provides tracking of a player’s last move time,
// position, yaw, and pitch, along with per-idle-streak AFK warning flags
// consumed by the hub's AFK evaluator.
type Movement struct {
	lastMoveTime time.Time
	lastPosition mgl64.Vec3
	lastRotation cube.Rotation

	warnedApproaching bool
	markedAFK         bool
	warnedFinal       bool

	mu sync.RWMutex
}

// NewMovement creates a new instance of Movement.
func NewMovement() *Movement {
	return &Movement{
		lastMoveTime: time.Now(),
		lastPosition: mgl64.Vec3{},
		lastRotation: cube.Rotation{},
	}
}

// LastMoveTime returns the time of the last movement.
func (m *Movement) LastMoveTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.lastMoveTime
}

// SetLastMoveTime updates the time of the last movement. Warning flags are
// reset so the next idle streak starts from scratch.
func (m *Movement) SetLastMoveTime(t time.Time) {
	m.mu.Lock()
	m.lastMoveTime = t
	m.warnedApproaching = false
	m.markedAFK = false
	m.warnedFinal = false
	m.mu.Unlock()
}

// WarnedApproaching reports whether the approaching-AFK soft warning has
// already been sent for the current idle streak.
func (m *Movement) WarnedApproaching() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.warnedApproaching
}

// SetWarnedApproaching marks the approaching-AFK soft warning as sent.
func (m *Movement) SetWarnedApproaching(v bool) {
	m.mu.Lock()
	m.warnedApproaching = v
	m.mu.Unlock()
}

// MarkedAFK reports whether the "you are now AFK" soft warning has already
// been sent for the current idle streak.
func (m *Movement) MarkedAFK() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.markedAFK
}

// SetMarkedAFK marks the "you are now AFK" soft warning as sent.
func (m *Movement) SetMarkedAFK(v bool) {
	m.mu.Lock()
	m.markedAFK = v
	m.mu.Unlock()
}

// WarnedFinal reports whether the near-capacity final warning has already
// been sent for the current idle streak.
func (m *Movement) WarnedFinal() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.warnedFinal
}

// SetWarnedFinal marks the near-capacity final warning as sent.
func (m *Movement) SetWarnedFinal(v bool) {
	m.mu.Lock()
	m.warnedFinal = v
	m.mu.Unlock()
}

// LastPosition returns the last recorded position.
func (m *Movement) LastPosition() mgl64.Vec3 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.lastPosition
}

// SetLastPosition updates the last recorded position.
func (m *Movement) SetLastPosition(pos mgl64.Vec3) {
	m.mu.Lock()
	m.lastPosition = pos
	m.mu.Unlock()
}

// LastRotation returns the last recorded rotation.
func (m *Movement) LastRotation() cube.Rotation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.lastRotation
}

// SetLastRotation updates the last recorded rotation.
func (m *Movement) SetLastRotation(rot cube.Rotation) {
	m.mu.Lock()
	m.lastRotation = rot
	m.mu.Unlock()
}
