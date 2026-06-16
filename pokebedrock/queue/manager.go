package queue

import (
	"container/heap"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/bossbar"
	"github.com/df-mc/dragonfly/server/world"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/authentication"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/internal"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

const (
	// highPriorityQueueThreshold is the position threshold for "Almost your turn" message.
	highPriorityQueueThreshold = 3

	// mediumPriorityQueueThreshold is the position threshold for "Short wait" message.
	mediumPriorityQueueThreshold = 10
)

// QueueManager is the global queue manager instance.
var QueueManager *Manager

func init() {
	QueueManager = NewManager()
}

// Transfer represents a player waiting to be transferred to a server.
type Transfer struct {
	player *player.Player
	entry  *Entry
	server *srv.Server
}

// Manager owns the priority queue of players waiting to join downstream
// servers and orchestrates per-tick transfer processing.
type Manager struct {
	mu sync.Mutex
	pq PriorityQueue

	pendingMu       sync.Mutex
	pendingBossBars map[*world.EntityHandle]struct{}
}

// NewManager creates a new queue manager.
func NewManager() *Manager {
	m := &Manager{
		pq:              PriorityQueue{},
		pendingBossBars: make(map[*world.EntityHandle]struct{}),
	}
	heap.Init(&m.pq)

	return m
}

// AddPlayer adds a player to the queue for a specific server. If the player
// is already queued they are removed first.
//
// Players are prioritized by rank, then by join time. A higher rank is
// always served before a lower rank, regardless of how long the lower-ranked
// player has waited.
func (m *Manager) AddPlayer(p *player.Player, r rank.Rank, server *srv.Server) {
	if server == nil {
		p.Message(locale.Translate("queue.nonexistent.server"))

		return
	}

	entry := &Entry{
		joinTime: time.Now(),
		handle:   p.H(),
		rank:     r,
		srv:      server,
	}

	m.mu.Lock()
	m.removeByHandleLocked(p.H())
	heap.Push(&m.pq, entry)
	m.mu.Unlock()

	m.queueAllBossBars()

	status := server.Status()
	switch {
	case status.Online && status.PlayerCount < status.MaxPlayerCount:
		p.Message(locale.Translate("queue.added.success", server.Name()))
	case !status.Online:
		p.Message(locale.Translate("queue.added.offline", server.Name()))
	default:
		p.Message(locale.Translate("queue.added.full",
			server.Name(), status.PlayerCount, status.MaxPlayerCount))
	}

	p.Message(locale.Translate("queue.priority.note"))
}

// RemovePlayer removes a player from the queue if present and
// clears their boss bar.
func (m *Manager) RemovePlayer(p *player.Player) {
	var serverName string

	m.mu.Lock()
	if removed := m.removeByHandleLocked(p.H()); removed != nil && removed.srv != nil {
		serverName = removed.srv.Name()
	}
	m.mu.Unlock()

	if serverName != "" {
		p.Messagef("%s", locale.Translate("queue.removed", serverName))
	}

	p.RemoveBossBar()
	m.queueAllBossBars()
}

// removeByHandleLocked removes the entry with the given handle, returning it
// if found. Caller must hold m.mu.
func (m *Manager) removeByHandleLocked(h *world.EntityHandle) *Entry {
	for i, entry := range m.pq {
		if entry != nil && entry.handle == h {
			removed := heap.Remove(&m.pq, i).(*Entry)

			return removed
		}
	}

	return nil
}

// removeEntryLocked removes the given entry by identity. Caller must hold
// m.mu. Indices are kept correct by heap.Swap, so this is safe even after
// other concurrent removals via heap operations.
func (m *Manager) removeEntryLocked(entry *Entry) {
	if entry == nil || entry.index < 0 || entry.index >= len(m.pq) {
		return
	}

	if m.pq[entry.index] != entry {
		// Index is stale; fall back to a linear scan.
		for i, e := range m.pq {
			if e == entry {
				heap.Remove(&m.pq, i)

				return
			}
		}

		return
	}

	heap.Remove(&m.pq, entry.index)
}

// NextPlayer pops the highest-priority entry from the queue, returning nil
// when the queue is empty.
func (m *Manager) NextPlayer() *Entry {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pq.Len() == 0 {
		return nil
	}

	return heap.Pop(&m.pq).(*Entry)
}

// snapshot returns a shallow copy of the current queue suitable for read-only
// iteration outside the lock. Entry pointers are shared so callers must not
// mutate Entry fields without re-locking.
func (m *Manager) snapshot() []*Entry {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]*Entry, len(m.pq))
	copy(out, m.pq)

	return out
}

// Update is invoked once per server tick. It performs all queue maintenance:
// removes stale entries, transfers up to one eligible player to their
// destination server, and schedules boss bar refreshes for affected players.
func (m *Manager) Update(tx *world.Tx) {
	queueSnap := m.snapshot()
	if len(queueSnap) == 0 {
		return
	}

	var (
		toRemove        []*Entry
		toTransfer      *Transfer
		invalidEntries  []*Entry
		invalidMessages []string
	)

	for _, entry := range queueSnap {
		if entry == nil || entry.handle == nil {
			toRemove = append(toRemove, entry)

			continue
		}

		ent, ok := entry.handle.Entity(tx)
		if !ok {
			toRemove = append(toRemove, entry)

			continue
		}

		p := ent.(*player.Player)

		s := entry.srv
		if s == nil {
			invalidEntries = append(invalidEntries, entry)
			invalidMessages = append(invalidMessages, "queue.destination.invalid")
			toRemove = append(toRemove, entry)
			_ = p

			continue
		}

		st := s.Status()
		if !st.Online {
			continue
		}
		if st.PlayerCount >= st.MaxPlayerCount {
			continue
		}
		if st.PlayerCount >= st.MaxPlayerCount-5 && entry.rank < rank.Admin {
			continue
		}

		toTransfer = &Transfer{player: p, entry: entry, server: s}
		toRemove = append(toRemove, entry)

		break
	}

	if len(toRemove) > 0 {
		m.mu.Lock()
		for _, entry := range toRemove {
			m.removeEntryLocked(entry)
		}
		m.mu.Unlock()
	}

	for i, entry := range invalidEntries {
		ent, ok := entry.handle.Entity(tx)
		if !ok {
			continue
		}
		if p, ok := ent.(*player.Player); ok {
			p.Message(locale.Translate(invalidMessages[i]))
		}
	}

	if toTransfer != nil {
		p, server := toTransfer.player, toTransfer.server

		p.Message(locale.Translate("connection.connecting", server.Name()))

		if err := p.Transfer(server.Address()); err != nil {
			p.Message(locale.Translate("connection.failed", err))
			m.mu.Lock()
			heap.Push(&m.pq, toTransfer.entry)
			m.mu.Unlock()
		} else {
			authentication.GlobalFactory().Set(p.Name(), p.XUID(), authentication.DefaultAuthDuration)
		}
	}

	if len(toRemove) > 0 || toTransfer != nil {
		m.queueAllBossBars()
	}

	m.processBossBarUpdates(tx, internal.ProcessingBatchSize)
}

// queueAllBossBars adds all current players in queue to the pending update set.
func (m *Manager) queueAllBossBars() {
	queueSnap := m.snapshot()
	if len(queueSnap) == 0 {
		return
	}

	m.pendingMu.Lock()
	for _, entry := range queueSnap {
		if entry != nil && entry.handle != nil {
			m.pendingBossBars[entry.handle] = struct{}{}
		}
	}
	m.pendingMu.Unlock()
}

// processBossBarUpdates processes up to maxCount pending boss bar updates.
// Position is computed in O(n) per player against the current queue snapshot,
// avoiding full sort spikes.
func (m *Manager) processBossBarUpdates(tx *world.Tx, maxCount int) {
	if maxCount <= 0 {
		return
	}

	batch := make([]*world.EntityHandle, 0, maxCount)

	m.pendingMu.Lock()
	for h := range m.pendingBossBars {
		batch = append(batch, h)
		delete(m.pendingBossBars, h)
		if len(batch) >= maxCount {
			break
		}
	}
	m.pendingMu.Unlock()

	if len(batch) == 0 {
		return
	}

	queueSnap := m.snapshot()

	for _, h := range batch {
		ent, ok := h.Entity(tx)
		if !ok {
			continue
		}

		p := ent.(*player.Player)
		position := positionFor(queueSnap, h)
		if position < 1 {
			continue
		}

		var waitMsg string
		switch {
		case position == 1:
			waitMsg = "You're next in line!"
		case position <= highPriorityQueueThreshold:
			waitMsg = "Almost your turn"
		case position <= mediumPriorityQueueThreshold:
			waitMsg = "Short wait"
		default:
			waitMsg = "Longer wait"
		}

		p.SendBossBar(bossbar.New(locale.Translate("queue.position", position, waitMsg)))
	}
}

// positionFor computes a player's 1-indexed priority position within the
// supplied queue snapshot, returning -1 if the player is not in the queue.
func positionFor(queue []*Entry, h *world.EntityHandle) int {
	var self *Entry
	for _, entry := range queue {
		if entry != nil && entry.handle == h {
			self = entry

			break
		}
	}
	if self == nil {
		return -1
	}

	position := 1
	for _, entry := range queue {
		if entry == nil || entry == self {
			continue
		}
		if entry.rank > self.rank ||
			(entry.rank == self.rank && entry.joinTime.Before(self.joinTime)) {
			position++
		}
	}

	return position
}

// GetQueuePosition returns a player's 1-indexed position in the queue, or -1
// if the player is not queued.
func (m *Manager) GetQueuePosition(p *player.Player) int {
	return positionFor(m.snapshot(), p.H())
}

// IsPlayerInQueue returns true if the given player has an entry in the queue.
func (m *Manager) IsPlayerInQueue(p *player.Player) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, entry := range m.pq {
		if entry != nil && entry.handle == p.H() {
			return true
		}
	}

	return false
}

// QueueSize returns the number of entries currently in the queue.
func (m *Manager) QueueSize() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.pq.Len()
}

// Queue returns a snapshot of the current queue for read-only use.
//
// Deprecated callers that mutated the returned slice are no longer
// supported; mutations will not be reflected in the manager.
func (m *Manager) Queue() []*Entry {
	return m.snapshot()
}
