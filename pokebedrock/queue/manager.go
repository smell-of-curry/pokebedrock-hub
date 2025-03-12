package queue

import (
	"container/heap"
	"sort"
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/bossbar"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

// QueueManager ...
var QueueManager *Manager

// init ...
func init() {
	QueueManager = NewManager()
}

// QueueTransfer represents a player waiting to be transferred to a server
type QueueTransfer struct {
	player *player.Player
	entry  *Entry
	server *srv.Server
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
//
// Note about queue priority:
// Players are prioritized by rank first, then by join time.
// This means a player with a higher rank (e.g., Admin) will always be placed
// ahead of players with lower ranks (e.g., Trainer), regardless of how long
// the lower-ranked players have been waiting.
func (m *Manager) AddPlayer(p *player.Player, r rank.Rank, srv *srv.Server) {
	// First check if player is already in queue
	m.RemovePlayer(p)

	// Verify server exists
	if srv == nil {
		p.Message(locale.Translate("queue.nonexistent.server"))
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

	// Inform player about their queue status and explain priority system
	status := srv.Status()
	if status.Online && status.PlayerCount < status.MaxPlayerCount {
		p.Message(locale.Translate("queue.added.success", srv.Name()))
	} else if !status.Online {
		p.Message(locale.Translate("queue.added.offline", srv.Name()))
	} else {
		p.Message(locale.Translate("queue.added.full",
			srv.Name(), status.PlayerCount, status.MaxPlayerCount))
	}

	// Explain queue priority system to the player
	p.Message(locale.Translate("queue.priority.note"))
}

// RemovePlayer ...
func (m *Manager) RemovePlayer(p *player.Player) {
	for i, entry := range m.Queue() {
		if entry.handle == p.H() {
			m.RemoveFromQueue(i)
			p.Messagef(locale.Translate("queue.removed", entry.srv.Name()))
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

	// Instead of modifying the queue during iteration,
	// we'll track changes and apply them afterward
	var entriesToRemove []int
	var playersToTransfer []*QueueTransfer

	// First pass: check queue entries and mark for removal/transfer
	for i, entry := range queue {
		// Skip already marked entries
		if i < 0 || i >= len(queue) {
			continue
		}

		// Check for nil entries
		if entry == nil || entry.handle == nil {
			entriesToRemove = append(entriesToRemove, i)
			continue
		}

		// Verify p still exists
		ent, ok := entry.handle.Entity(tx)
		if !ok {
			entriesToRemove = append(entriesToRemove, i)
			continue
		}
		p := ent.(*player.Player)

		// Verify server still exists and is valid
		s := entry.srv
		if s == nil {
			entriesToRemove = append(entriesToRemove, i)
			p.Message(locale.Translate("queue.destination.invalid"))
			continue
		}

		// Check server status
		st := s.Status()
		if !st.Online {
			// Skip offline servers but keep p in queue
			continue
		}

		// Check if there's capacity in the server
		if st.PlayerCount >= st.MaxPlayerCount {
			// Server is full, keep p in queue
			continue
		}

		// Admin bypass - allow admins to join immediately regardless of server capacity
		// Others still need to wait for available slots
		if st.PlayerCount >= st.MaxPlayerCount-5 && entry.rank < rank.Admin {
			// Server is near capacity, only admins can bypass
			continue
		}

		// Mark for transfer
		playersToTransfer = append(playersToTransfer, &QueueTransfer{
			player: p,
			entry:  entry,
			server: s,
		})
		entriesToRemove = append(entriesToRemove, i)

		// Only process one transfer per tick
		break
	}

	// Second pass: remove entries marked for removal (in reverse order to maintain indices)
	sort.Sort(sort.Reverse(sort.IntSlice(entriesToRemove)))
	for _, i := range entriesToRemove {
		if i >= 0 && i < len(m.Queue()) {
			m.RemoveFromQueue(i)
		}
	}

	// Third pass: process transfers
	for _, transfer := range playersToTransfer {
		// Notify the player they're being transferred
		transfer.player.Message(locale.Translate("connection.connecting", transfer.server.Name()))

		// Transfer the player
		if err := transfer.player.Transfer(transfer.server.Address()); err != nil {
			// If transfer fails, add player back to queue
			transfer.player.Message(locale.Translate("connection.failed", err))
			m.AddToQueue(transfer.entry)
		}
	}

	// Update boss bars for remaining players if any changes were made
	if len(entriesToRemove) > 0 {
		m.updateBossBars(tx)
	}
}

// updateBossBars updates the boss bars for all players in the queue showing their position.
func (m *Manager) updateBossBars(tx *world.Tx) {
	queue := m.Queue()
	length := len(queue)
	if length == 0 {
		return
	}

	// Create a sorted copy of the queue to accurately show positions
	// We need to do this because the underlying heap's order doesn't necessarily match the priority order
	sortedEntries := make([]*Entry, length)
	copy(sortedEntries, queue)

	// Sort entries by the same priority rules as the queue
	sort.Slice(sortedEntries, func(i, j int) bool {
		// Sort in priority order (reverse of what Less() does since we want highest priority first)
		if sortedEntries[i].rank == sortedEntries[j].rank {
			return sortedEntries[i].joinTime.Before(sortedEntries[j].joinTime)
		}
		return sortedEntries[i].rank > sortedEntries[j].rank
	})

	// Now update boss bars with accurate positions
	for i, entry := range sortedEntries {
		position := i + 1 // 1-indexed position

		ent, ok := entry.handle.Entity(tx)
		if !ok {
			continue
		}
		p := ent.(*player.Player)

		// Show estimated time based on position
		var waitMsg string
		if position == 1 {
			waitMsg = "You're next in line!"
		} else if position <= 3 {
			waitMsg = "Almost your turn"
		} else if position <= 10 {
			waitMsg = "Short wait"
		} else {
			waitMsg = "Longer wait"
		}

		bar := bossbar.New(locale.Translate("queue.position", position, waitMsg))
		p.SendBossBar(bar)
		position++
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
// The position is calculated based on the priority order (rank, then join time).
func (m *Manager) GetQueuePosition(p *player.Player) int {
	queue := m.Queue()
	if len(queue) == 0 {
		return -1
	}

	// First check if player is in the queue at all
	playerHandle := p.H()
	playerEntry := (*Entry)(nil)

	for _, entry := range queue {
		if entry.handle == playerHandle {
			playerEntry = entry
			break
		}
	}

	if playerEntry == nil {
		return -1 // Player not in queue
	}

	// Count how many players are ahead of this player based on priority
	position := 1
	for _, entry := range queue {
		if entry == playerEntry {
			continue // Skip self
		}

		// Same priority rules as queue - check if this entry has higher priority
		if entry.rank > playerEntry.rank ||
			(entry.rank == playerEntry.rank && entry.joinTime.Before(playerEntry.joinTime)) {
			position++
		}
	}

	return position
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
