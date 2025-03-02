package queue

import (
	"container/heap"
	"fmt"
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/bossbar"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

// QueueManager ...
var QueueManager *Manager

// init ...
func init() {
	QueueManager = NewManager()
}

// Manager ...
type Manager struct {
	queue atomic.Value[PriorityQueue]
}

// NewManager ...
func NewManager() *Manager {
	m := &Manager{
		queue: *atomic.NewValue(PriorityQueue{}),
	}
	q := m.queue.Load()
	heap.Init(&q)
	m.queue.Store(q)
	return m
}

// AddPlayer adds a player to the queue for a specific server.
// If the player is already in a queue, they are removed from it first.
func (m *Manager) AddPlayer(p *player.Player, r rank.Rank, srv *srv.Server) {
	// First check if player is already in queue
	m.RemovePlayer(p)

	// Verify server exists
	if srv == nil {
		p.Message(text.Colourf("<red>Cannot queue for a non-existent server.</red>"))
		return
	}

	// Create queue entry
	entry := &Entry{
		joinTime: time.Now(),
		handle:   p.H(),
		rank:     r,
		srv:      srv,
	}

	// Add to queue
	m.AddToQueue(entry)

	// Update UI
	m.updateBossBars(p.Tx())

	// Inform player
	status := srv.Status()
	if status.Online && status.PlayerCount < status.MaxPlayerCount {
		p.Message(text.Colourf("<green>You've been added to the queue for %s. The server has space available, you'll be transferred shortly.</green>", srv.Name()))
	} else if !status.Online {
		p.Message(text.Colourf("<yellow>You've been added to the queue for %s. The server is currently offline. You'll be transferred when it comes online.</yellow>", srv.Name()))
	} else {
		p.Message(text.Colourf("<yellow>You've been added to the queue for %s. The server is currently full (%d/%d players). You'll be transferred when space becomes available.</yellow>",
			srv.Name(), status.PlayerCount, status.MaxPlayerCount))
	}
}

// RemovePlayer ...
func (m *Manager) RemovePlayer(p *player.Player) {
	for i, entry := range m.Queue() {
		if entry.handle == p.H() {
			m.RemoveFromQueue(i)
			p.Messagef(text.Colourf("<red>You've been removed from the queue for %s.</red>", entry.srv.Name()))
			break
		}
	}
	p.RemoveBossBar()
	m.updateBossBars(p.Tx())
}

// NextPlayer ...
func (m *Manager) NextPlayer() *Entry {
	queue := m.Queue()
	if len(queue) == 0 {
		return nil
	}

	entry := heap.Pop(&queue).(*Entry)
	m.queue.Store(queue)
	return entry
}

// Update ...
func (m *Manager) Update(tx *world.Tx) {
	queue := m.Queue()
	if len(queue) == 0 {
		return
	}

	// Check each player in the queue in order (respecting priority)
	for i, entry := range queue {
		// Verify player still exists
		p, ok := entry.handle.Entity(tx)
		if !ok {
			m.RemoveFromQueue(i)
			continue
		}
		player := p.(*player.Player)

		// Verify server still exists and is valid
		s := entry.srv
		if s == nil {
			m.RemoveFromQueue(i)
			player.Message(text.Colourf("<red>Your queue destination no longer exists.</red>"))
			continue
		}

		// Check server status
		st := s.Status()
		if !st.Online {
			// Skip offline servers but keep player in queue
			continue
		}

		// Check if there's capacity in the server
		if st.PlayerCount >= st.MaxPlayerCount {
			// Server is full, keep player in queue
			continue
		}

		// Admin bypass - allow admins to join immediately regardless of server capacity
		// Others still need to wait for available slots
		if st.PlayerCount >= st.MaxPlayerCount-5 && entry.rank < rank.Admin {
			// Server is near capacity, only admins can bypass
			continue
		}

		// Remove from queue before transfer to avoid race conditions
		m.RemoveFromQueue(i)

		// Notify the player they're being transferred
		player.Message(text.Colourf("<green>Connecting you to %s...</green>", s.Name()))

		// Transfer the player
		if err := player.Transfer(s.Address()); err != nil {
			// If transfer fails, add player back to queue
			player.Message(text.Colourf("<red>Connection failed: %v. You've been placed back in queue.</red>", err))
			m.AddToQueue(entry)
		}

		// Update boss bars for remaining players
		m.updateBossBars(tx)
		break // Only process one transfer per tick
	}
}

// updateBossBars updates the boss bars for all players in the queue showing their position.
func (m *Manager) updateBossBars(tx *world.Tx) {
	queue := m.Queue()
	length := len(queue)
	if length == 0 {
		return
	}

	pos := 1
	for _, entry := range queue {
		ent, ok := entry.handle.Entity(tx)
		if !ok {
			continue
		}
		player := ent.(*player.Player)

		// Show estimated time based on position
		var waitMsg string
		if pos == 1 {
			waitMsg = "You're next in line!"
		} else if pos <= 3 {
			waitMsg = "Almost your turn"
		} else if pos <= 10 {
			waitMsg = "Short wait"
		} else {
			waitMsg = "Longer wait"
		}

		bar := bossbar.New(fmt.Sprintf("Queue position: #%d - %s", pos, waitMsg))
		player.SendBossBar(bar)
		pos++
	}
}

// AddToQueue ...
func (m *Manager) AddToQueue(entry *Entry) {
	q := m.Queue()
	heap.Push(&q, entry)
	m.queue.Store(q)
}

// RemoveFromQueue ...
func (m *Manager) RemoveFromQueue(i int) {
	q := m.Queue()
	heap.Remove(&q, i)
	m.queue.Store(q)
}

// Queue ...
func (m *Manager) Queue() PriorityQueue {
	return m.queue.Load()
}

// GetQueuePosition returns a player's position in the queue, or -1 if not in queue.
func (m *Manager) GetQueuePosition(p *player.Player) int {
	queue := m.Queue()
	for i, entry := range queue {
		if entry.handle == p.H() {
			return i + 1 // 1-indexed position for user display
		}
	}
	return -1 // Not in queue
}

// IsPlayerInQueue checks if a player is already in the queue.
func (m *Manager) IsPlayerInQueue(p *player.Player) bool {
	queue := m.Queue()
	for _, entry := range queue {
		if entry.handle == p.H() {
			return true
		}
	}
	return false
}

// QueueSize returns the current size of the queue.
func (m *Manager) QueueSize() int {
	return m.Queue().Len()
}
