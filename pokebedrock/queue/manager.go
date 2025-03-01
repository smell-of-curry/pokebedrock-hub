package queue

import (
	"container/heap"
	"fmt"
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/bossbar"
	"github.com/df-mc/dragonfly/server/world"
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

// AddPlayer ...
func (m *Manager) AddPlayer(p *player.Player, r rank.Rank, srv *srv.Server) {
	entry := &Entry{
		joinTime: time.Now(),
		handle:   p.H(),
		rank:     r,
		srv:      srv,
	}
	m.AddToQueue(entry)
	m.updateBossBars(p.Tx())
}

// RemovePlayer ...
func (m *Manager) RemovePlayer(p *player.Player) {
	for i, entry := range m.Queue() {
		if entry.handle == p.H() {
			m.RemoveFromQueue(i)
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

	next := m.NextPlayer()
	if next != nil {
		ent, ok := next.handle.Entity(tx)
		if !ok {
			return
		}

		s := next.srv
		st := s.Status()
		if !st.Online {
			m.AddToQueue(next)
			return
		}

		p := ent.(*player.Player)
		if st.PlayerCount < st.MaxPlayerCount-5 {
			if next.rank >= rank.Admin {
				_ = p.Transfer(s.Address())
				m.updateBossBars(tx)
				return
			}
			m.AddToQueue(next)
			return
		}

		_ = p.Transfer(s.Address())
		m.updateBossBars(tx)
	}
}

// updateBossBars ...
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

		bar := bossbar.New(fmt.Sprintf("Your position in queue: #%d", pos))
		ent.(*player.Player).SendBossBar(bar)
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
