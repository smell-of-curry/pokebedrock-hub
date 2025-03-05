package handler

import (
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// Global logger for rank-related operations
var rankLogger = slog.Default()

// Channel for async rank updates and shutdown
var (
	rankUpdateCh = make(chan rankUpdate, 100)
)

// SetRankLogger sets the logger used for rank operations
func SetRankLogger(logger *slog.Logger) {
	rankLogger = logger
}

// rankUpdate represents a rank update request for a player
type rankUpdate struct {
	handler *PlayerHandler
	handle  *world.EntityHandle
	xuid    string
	doneCh  chan struct{}
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
		case <-update.doneCh:
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
		update.handler.SetRanks(ranks)

		// Signal completion
		close(update.doneCh)

		update.handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
			p, ok := e.(*player.Player)
			if !ok {
				return
			}

			highestRank := update.handler.HighestRank()
			nameTag := highestRank.NameTag(p.Name())
			p.SetNameTag(nameTag)

			rankUpdateMessage := text.Colourf("<green>Your highest rank has been synced to '%s'!</green>", highestRank.Name())
			p.SendTip(rankUpdateMessage) // Send to update in action bar
			p.Message(rankUpdateMessage) // Send to keep player notified if they exit out.
		})
	}
}

// LoadRanksAsync queues an asynchronous fetch of player ranks
// If player is provided, their nametag will be updated once ranks are fetched
func (h *PlayerHandler) LoadRanksAsync(xuid string, handle *world.EntityHandle) {
	// Create a buffered channel to prevent goroutine leak
	doneCh := make(chan struct{}, 1)

	select {
	case rankUpdateCh <- rankUpdate{
		handler: h,
		handle:  handle,
		xuid:    xuid,
		doneCh:  doneCh,
	}:
		handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
			p, ok := e.(*player.Player)
			if !ok {
				return
			}
			p.SendTip("Fetching your rank")
		})
	default:
		// Channel full, log warning but continue
		handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
			p, ok := e.(*player.Player)
			if !ok {
				return
			}
			p.SendTip(text.Colourf("<red>Rank update queue is full, please try again later.</red>"))
		})
		return
	}

	// Start a goroutine to handle the timeout and tips
	timeout := time.After(5 * time.Second) // Increased timeout
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	i := 0
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
				handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
					p, ok := e.(*player.Player)
					if !ok {
						return
					}
					cycle := []int{1, 2, 3}
					p.SendTip(fmt.Sprintf("Fetching your rank%s", strings.Repeat(".", cycle[i%len(cycle)])))
				})
			}
			i++
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
					p.SendTip(text.Colourf("<red>Rank fetch timed out, try again later.</red>"))
				})
				return
			}
		}
	}
}

// fetchRanks is a helper function that fetches ranks from API.
func fetchRanks(xuid string) []rank.Rank {
	roles, err := rank.GlobalService().RolesOfXUID(xuid)
	if err != nil {
		// Log the error
		rank.RolesError(rankLogger, xuid, err)

		// Use default rank
		return []rank.Rank{rank.Trainer}
	}

	// API request successful, get ranks
	ranks := rank.RolesToRanks(roles)
	if len(ranks) == 0 {
		ranks = []rank.Rank{rank.Trainer}
	}

	return ranks
}

// SetRanks updates the player's ranks and sorts them.
func (h *PlayerHandler) SetRanks(ranks []rank.Rank) {
	if len(ranks) == 0 {
		ranks = []rank.Rank{rank.Trainer}
	}

	h.rankMu.Lock()
	h.ranks = ranks
	h.rankMu.Unlock()
	h.sortRanks()
}

// HighestRank returns the player's highest rank.
func (h *PlayerHandler) HighestRank() rank.Rank {
	h.rankMu.Lock()
	defer h.rankMu.Unlock()

	if len(h.ranks) == 0 {
		return rank.Trainer
	}
	return h.ranks[len(h.ranks)-1]
}

// Ranks returns a copy of the player's ranks.
func (h *PlayerHandler) Ranks() []rank.Rank {
	h.rankMu.Lock()
	defer h.rankMu.Unlock()

	// Return a copy to prevent external modifications
	ranksCopy := make([]rank.Rank, len(h.ranks))
	copy(ranksCopy, h.ranks)
	return ranksCopy
}

// HasRank checks if the player has a specific rank.
func (h *PlayerHandler) HasRank(r rank.Rank) bool {
	h.rankMu.Lock()
	defer h.rankMu.Unlock()

	return slices.Contains(h.ranks, r)
}

// HasRankOrHigher checks if the player has the specified rank or a higher one.
func (h *PlayerHandler) HasRankOrHigher(r rank.Rank) bool {
	return h.HighestRank() >= r
}

// sortRanks sorts the ranks in ascending order.
func (h *PlayerHandler) sortRanks() {
	h.rankMu.Lock()
	defer h.rankMu.Unlock()

	sort.SliceStable(h.ranks, func(i, j int) bool {
		return h.ranks[i] < h.ranks[j]
	})
}
