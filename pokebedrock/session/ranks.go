package session

import (
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// Channel for async rank updates and shutdown
var (
	rankUpdateCh = make(chan rankUpdate, 100)
)

// rankUpdate represents a rank update request for a player
type rankUpdate struct {
	xuid string

	handle *world.EntityHandle
	ranks  *Ranks

	ch chan struct{}
}

// init starts the background rank worker and cache cleanup
func init() {
	go rankWorker()
}

// rankWorker processes rank updates in the background
func rankWorker() {
	for update := range rankUpdateCh {
		// Check if the update timed out
		select {
		case <-update.ch:
			continue
		default:
		}

		// Fetch the player's ranks
		ranks := fetchRanks(update.xuid)

		// Ensure the player is still online
		if update.handle == nil {
			continue
		}

		// Update the player's ranks
		update.ranks.SetRanks(ranks)

		// Signal completion
		close(update.ch)

		update.handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
			p, ok := e.(*player.Player)
			if !ok {
				return
			}

			highestRank := update.ranks.HighestRank()
			nameTag := highestRank.NameTag(p.Name())
			p.SetNameTag(nameTag)

			msg := locale.Translate("rank.synced", highestRank.Name())
			p.SendTip(msg) // Send to update in action bar
			p.Message(msg) // Send to keep player notified if they exit out.
		})
	}
}

// Ranks ...
type Ranks struct {
	rankMu sync.Mutex
	ranks  []rank.Rank

	lastRankFetch atomic.Value[time.Time]
}

// NewRanks ...
func NewRanks() *Ranks {
	r := &Ranks{
		ranks: make([]rank.Rank, 0),
	}
	r.lastRankFetch.Store(time.Time{})
	return r
}

// Load queues an asynchronous fetch of player ranks
// If player is provided, their name tag will be updated once ranks are fetched
func (r *Ranks) Load(xuid string, handle *world.EntityHandle) {
	// Create a buffered channel to prevent goroutine leak
	doneCh := make(chan struct{}, 1)

	select {
	case rankUpdateCh <- rankUpdate{
		ranks:  r,
		handle: handle,
		xuid:   xuid,
		ch:     doneCh,
	}:
		handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
			p, ok := e.(*player.Player)
			if !ok {
				return
			}
			p.SendTip(locale.Translate("rank.fetching"))
		})
	default:
		// Channel full, log warning but continue
		handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
			p, ok := e.(*player.Player)
			if !ok {
				return
			}
			p.SendTip(locale.Translate("rank.update.queue.full"))
		})
		return
	}

	// Start a goroutine to handle the timeout and tips
	timeout := time.After(5 * time.Second) // Increased timeout
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-doneCh:
			return
		case <-ticker.C:
			select {
			case <-doneCh:
				return
			default:
				if handle == nil {
					ticker.Stop()
					return
				}
			}
		case <-timeout:
			select {
			case <-doneCh:
				return
			default:
				if handle == nil {
					return
				}
				doneCh <- struct{}{} // Close the channel to signal timeout
				handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
					p, ok := e.(*player.Player)
					if !ok {
						return
					}
					p.SendTip(locale.Translate("rank.fetch.timeout"))
				})
				return
			}
		}
	}
}

// fetchRanks retrieves the ranks associated with the given XUID from the API.
func fetchRanks(xuid string) []rank.Rank {
	log := slog.Default()
	roles, err := rank.GlobalService().RolesOfXUID(xuid)
	if err != nil {
		// Log the error
		rank.RolesError(log, xuid, err)
		// Use default rank
		return []rank.Rank{rank.UnLinked}
	}

	// API request successful, get ranks
	ranks := rank.RolesToRanks(roles)
	if len(ranks) == 0 {
		// Player has no valid roles that map to ranks, shouldn't be possible so we will just map to Trainer
		ranks = []rank.Rank{rank.Trainer}
		// Log the error
		rank.RolesError(log, xuid, fmt.Errorf("player has account linked but no valid roles"))
	}
	return ranks
}

// SetRanks updates the players ranks and sorts them.
func (r *Ranks) SetRanks(ranks []rank.Rank) {
	r.rankMu.Lock()
	r.ranks = ranks
	r.rankMu.Unlock()
	r.sortRanks()
}

// HighestRank returns the players highest rank.
func (r *Ranks) HighestRank() rank.Rank {
	r.rankMu.Lock()
	defer r.rankMu.Unlock()
	if len(r.ranks) == 0 {
		return rank.UnLinked
	}
	return r.ranks[len(r.ranks)-1]
}

// Ranks returns a copy of the players ranks.
func (r *Ranks) Ranks() []rank.Rank {
	r.rankMu.Lock()
	defer r.rankMu.Unlock()
	ranksCopy := append([]rank.Rank(nil), r.ranks...)
	return ranksCopy
}

// HasRank checks if the player has a specific rank.
func (r *Ranks) HasRank(ra rank.Rank) bool {
	r.rankMu.Lock()
	defer r.rankMu.Unlock()
	return slices.Contains(r.ranks, ra)
}

// HasRankOrHigher checks if the player has the specified rank or a higher one.
func (r *Ranks) HasRankOrHigher(ra rank.Rank) bool {
	return r.HighestRank() >= ra
}

// sortRanks sorts the ranks in ascending order.
func (r *Ranks) sortRanks() {
	r.rankMu.Lock()
	defer r.rankMu.Unlock()
	sort.SliceStable(r.ranks, func(i, j int) bool {
		return r.ranks[i] < r.ranks[j]
	})
}

// LastRankFetch ...
func (r *Ranks) LastRankFetch() time.Time {
	return r.lastRankFetch.Load()
}

// SetLastRankFetch ...
func (r *Ranks) SetLastRankFetch(t time.Time) {
	r.lastRankFetch.Store(t)
}
