package session

import (
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// Channel for async rank updates and shutdown
var (
	rankUpdateCh = make(chan rankUpdate, 100)
	// rankLoadQueue is a buffered channel for loading player ranks
	rankLoadQueue = make(chan rankLoadRequest, 50)
	// Used to signal worker shutdown
	rankLoadWorkerShutdown = make(chan struct{})
)

// StopRankChannel closes the rank update channel and ensures no more rank updates
// will be processed. This should be called during server shutdown.
func StopRankChannel() {
	// Close the channel first to prevent new updates from being queued
	close(rankUpdateCh)

	// Allow time for any in-progress updates to complete
	timeout := time.NewTimer(3 * time.Second)
	<-timeout.C
}

// rankUpdate represents a rank update request for a player.
// It contains all necessary data to process a rank update asynchronously.
type rankUpdate struct {
	xuid string

	handle *world.EntityHandle
	ranks  *Ranks

	ch chan struct{}
}

// rankLoadRequest represents a queued request to load player ranks
type rankLoadRequest struct {
	xuid   string
	handle *world.EntityHandle
	ranks  *Ranks
}

// init starts the background rank worker and cache cleanup goroutines.
// These goroutines handle rank updates and cleanup tasks in the background.
func init() {
	go rankWorker()
	go rankLoadWorker()
}

// updatePlayer sends a colored message to the player, sets their ranks, and closes the update's done channel.
func updatePlayer(update rankUpdate, message string, color string) {
	// Ensure the player is still online
	if update.handle == nil {
		return
	}

	// Get execute permission
	update.handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
		p, ok := e.(*player.Player)
		if !ok {
			return
		}
		msg := text.Colourf("<%s>%s</%s>", color, message, color)
		p.SendTip(msg)
		p.Message(msg)
	})

	// Notify the worker that the update is done
	close(update.ch)
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

		// Ensure the player is still online
		if update.handle == nil {
			continue
		}

		// Fetch the player's roles
		roles, err := rank.GlobalService().RolesOfXUID(update.xuid)
		if err != nil {
			update.ranks.SetRanks([]rank.Rank{rank.UnLinked})
			updatePlayer(update, rank.RolesError(err), "red")
			continue
		}

		// API request successful, map roles to ranks
		ranks := rank.RolesToRanks(roles)
		if len(ranks) == 0 {
			// Player has no valid roles that map to ranks, shouldn't be possible so we will just map to Trainer
			ranks = []rank.Rank{rank.Trainer}
		}

		// Ensure the player is still online
		if update.handle == nil {
			continue
		}

		// Update the player's ranks
		update.ranks.SetRanks(ranks)

		highestRank := update.ranks.HighestRank()
		rankUpdateMessage := locale.Translate("rank.synced", highestRank.Name())
		updatePlayer(update, rankUpdateMessage, "green")

		// Update the player's nametag
		update.handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
			p, ok := e.(*player.Player)
			if !ok {
				return
			}

			nameTag := highestRank.NameTag(p.Name())
			p.SetNameTag(nameTag)
		})
	}
}

// rankLoadWorker processes rank loading requests with rate limiting
func rankLoadWorker() {
	// Create a semaphore to limit concurrent API requests
	semaphore := make(chan struct{}, 3) // Allow 3 concurrent requests

	for {
		select {
		case <-rankLoadWorkerShutdown:
			return
		case req, ok := <-rankLoadQueue:
			if !ok {
				return
			}

			// Acquire semaphore slot
			semaphore <- struct{}{}

			go func(req rankLoadRequest) {
				defer func() {
					// Release semaphore slot when done
					<-semaphore
				}()

				// Use the original Load method to load ranks
				req.ranks.Load(req.xuid, req.handle)
			}(req)
		}
	}
}

// Ranks represents a struct to manage player ranks and rank fetching times.
type Ranks struct {
	rankMu sync.Mutex
	ranks  []rank.Rank

	lastRankFetch atomic.Value[time.Time]
}

// NewRanks initializes and returns a new instance of Ranks.
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

	timeout := time.After(5 * time.Second)
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

// SetRanks updates the player's ranks and sorts them in ascending order.
// This ensures that the highest rank is always last in the slice.
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
	ranksCopy := slices.Clone(r.ranks)
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

// LastRankFetch returns the last time the rank was fetched.
func (r *Ranks) LastRankFetch() time.Time {
	return r.lastRankFetch.Load()
}

// SetLastRankFetch sets the last time the rank was fetched.
func (r *Ranks) SetLastRankFetch(t time.Time) {
	r.lastRankFetch.Store(t)
}

// QueueLoad adds a rank loading request to the queue
func (r *Ranks) QueueLoad(xuid string, handle *world.EntityHandle) {
	// Avoid blocking if queue is full
	select {
	case rankLoadQueue <- rankLoadRequest{
		xuid:   xuid,
		handle: handle,
		ranks:  r,
	}:
		// Successfully queued
		if handle != nil {
			handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
				p, ok := e.(*player.Player)
				if !ok {
					return
				}
				p.SendTip(locale.Translate("rank.queue.added"))
			})
		}
	default:
		// Queue is full
		if handle != nil {
			handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
				p, ok := e.(*player.Player)
				if !ok {
					return
				}
				p.SendTip(locale.Translate("rank.update.queue.full"))
			})
		}
	}
}

// StopRankLoadWorker stops the rank loading worker
func StopRankLoadWorker() {
	close(rankLoadWorkerShutdown)
}
