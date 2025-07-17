package session

import (
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

// Movement provides tracking of a player’s last move time,
// position, yaw, and pitch.
type Movement struct {
	lastMoveTime time.Time
	lastPosition mgl64.Vec3
	lastRotation cube.Rotation
	mu           sync.RWMutex
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

// SetLastMoveTime updates the time of the last movement.
func (m *Movement) SetLastMoveTime(t time.Time) {
	m.mu.Lock()
	m.lastMoveTime = t
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
