package queue

import (
	"fmt"
	"time"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

// Entry represents a player waiting in the queue to join a server.
// Entries are prioritized by rank first, then by join time for equal ranks.
type Entry struct {
	joinTime time.Time           // When the player joined the queue
	index    int                 // Index in the heap, used by heap.Interface
	handle   *world.EntityHandle // Handle to the player entity
	rank     rank.Rank           // Player's rank for priority determination
	srv      *srv.Server         // Target server to connect to
}

// String returns a string representation of the entry for debugging.
func (e *Entry) String() string {
	serverName := "unknown"
	if e.srv != nil {
		serverName = e.srv.Name()
	}

	return fmt.Sprintf("Entry{player: %s, rank: %s, server: %s, joined: %s}",
		e.handle.UUID(), e.rank.Name(), serverName, e.joinTime.Format(time.RFC3339))
}
