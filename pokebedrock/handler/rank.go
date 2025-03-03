package handler

import (
	"log/slog"
	"math"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// Global logger for rank-related operations
var rankLogger = slog.Default()

// SetRankLogger sets the logger used for rank operations
func SetRankLogger(logger *slog.Logger) {
	rankLogger = logger
}

// ranksCache is a temporary cache of player ranks to reduce API calls
var (
	ranksCache     = make(map[string]rankCacheEntry)
	ranksCacheMu   sync.RWMutex
	ranksCacheTTL  = 10 * time.Minute
	cachePurgeTime = time.Now()

	// Channel for async rank updates
	rankUpdateCh = make(chan rankUpdate, 100)
)

// rankUpdate represents a rank update request for a player
type rankUpdate struct {
	handler *PlayerHandler
	handle  *world.EntityHandle
	xuid    string
}

// init starts the background rank worker
func init() {
	go rankWorker()
}

// rankWorker processes rank updates in the background
func rankWorker() {
	for update := range rankUpdateCh {
		// Process the update
		ranks := fetchRanks(update.xuid)

		// Update the player's ranks
		update.handler.SetRanks(ranks)

		if update.handle != nil {
			update.handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
				p := e.(*player.Player)

				nameTag := update.handler.HighestRank().NameTag(p.Name())
				p.SetNameTag(nameTag)

				// TODO: And tell Player there rank has been updated
			})
		}
	}
}

// rankCacheEntry ...
type rankCacheEntry struct {
	ranks    []rank.Rank
	expiry   time.Time
	attempts int
}

// loadRanks loads the ranks for a player synchronously, but only from cache
// If not in cache, it assigns a default rank and triggers an async fetch
func (h *PlayerHandler) loadRanks(xuid string) {
	// Try to purge expired cache entries occasionally
	if time.Since(cachePurgeTime) > time.Minute {
		go purgeExpiredCacheEntries()
		cachePurgeTime = time.Now()
	}

	// Check cache first
	ranksCacheMu.RLock()
	entry, found := ranksCache[xuid]
	ranksCacheMu.RUnlock()

	// If found in cache and not expired, use cached ranks
	if found && time.Now().Before(entry.expiry) {
		h.SetRanks(entry.ranks)
		return
	}

	// If in cache but expired, use cached ranks temporarily
	if found {
		h.SetRanks(entry.ranks)
	} else {
		// Not in cache, set default rank temporarily
		h.SetRanks([]rank.Rank{rank.Trainer})
	}

	// Queue async fetch of latest ranks
	h.LoadRanksAsync(xuid, nil)
}

// LoadRanksAsync queues an asynchronous fetch of player ranks
// If player is provided, their nametag will be updated once ranks are fetched
func (h *PlayerHandler) LoadRanksAsync(xuid string, handle *world.EntityHandle) {
	select {
	case rankUpdateCh <- rankUpdate{
		handler: h,
		handle:  handle,
		xuid:    xuid,
	}:
		// Request queued successfully
	default:
		// Channel full, log warning but continue
		rankLogger.Warn("Rank update queue is full, skipping update", "xuid", xuid)
	}
}

// fetchRanks is a helper function that fetches ranks from API or cache
// This runs in the background worker and doesn't block the main thread
func fetchRanks(xuid string) []rank.Rank {
	// Check cache first
	ranksCacheMu.RLock()
	entry, found := ranksCache[xuid]
	ranksCacheMu.RUnlock()

	// If found in cache and not expired, use cached ranks
	if found && time.Now().Before(entry.expiry) {
		return entry.ranks
	}

	// If not in cache or expired, fetch from API
	roles, err := rank.GlobalService().RolesOfXUID(xuid)
	if err != nil {
		// Log the error
		rank.RolesError(rankLogger, xuid, err)

		// Use cached ranks if available (even if expired)
		if found {
			// Update the cache with extended expiry to avoid hammering the API
			entry.attempts++
			backoffDuration := time.Duration(entry.attempts*30) * time.Second
			maxDuration := 5 * time.Minute
			backoffTime := time.Duration(math.Min(float64(backoffDuration), float64(maxDuration)))
			entry.expiry = time.Now().Add(backoffTime)

			ranksCacheMu.Lock()
			ranksCache[xuid] = entry
			ranksCacheMu.Unlock()

			return entry.ranks
		}

		// If not in cache and API fails, use default rank
		return []rank.Rank{rank.Trainer}
	}

	// API request successful, get ranks
	ranks := rank.RolesToRanks(roles)
	if len(ranks) == 0 {
		ranks = []rank.Rank{rank.Trainer}
	}

	// Update cache with freshly fetched ranks
	ranksCacheMu.Lock()
	ranksCache[xuid] = rankCacheEntry{
		ranks:    ranks,
		expiry:   time.Now().Add(ranksCacheTTL),
		attempts: 0,
	}
	ranksCacheMu.Unlock()

	return ranks
}

// RefreshRanks forces a refresh of the player's ranks from the API.
// This is useful when ranks might have changed externally.
func (h *PlayerHandler) RefreshRanks(p *player.Player) {
	if p == nil {
		return
	}

	// Clear from cache first
	ranksCacheMu.Lock()
	delete(ranksCache, p.XUID())
	ranksCacheMu.Unlock()

	// Load ranks asynchronously
	h.LoadRanksAsync(p.XUID(), p.H())
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

// purgeExpiredCacheEntries removes expired entries from the ranks cache.
func purgeExpiredCacheEntries() {
	now := time.Now()
	ranksCacheMu.Lock()
	defer ranksCacheMu.Unlock()

	for xuid, entry := range ranksCache {
		if now.After(entry.expiry) {
			delete(ranksCache, xuid)
		}
	}
}
