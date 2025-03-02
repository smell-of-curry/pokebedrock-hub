package handler

import (
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/data"
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
)

type rankCacheEntry struct {
	ranks    []rank.Rank
	expiry   time.Time
	attempts int
}

// loadRanks loads the ranks for a player based on their XUID.
// It will try to load from cache first, and if not available or expired, fetch from the API.
// If the API request fails, it will use cached values if available (even if expired) or default to Trainer rank.
func (h *PlayerHandler) loadRanks(xuid string) {
	// Try to purge expired cache entries occasionally (not on every call to avoid overhead)
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

	// If not in cache or expired, fetch from API
	roles, err := data.Roles(xuid)
	if err != nil {
		// Log the error
		data.LogRolesError(rankLogger, xuid, err)

		// Use cached ranks if available (even if expired)
		if found {
			h.SetRanks(entry.ranks)

			// Update the cache with extended expiry to avoid hammering the API
			entry.attempts++
			backoffTime := time.Duration(entry.attempts*30) * time.Second
			if backoffTime > 5*time.Minute {
				backoffTime = 5 * time.Minute
			}
			entry.expiry = time.Now().Add(backoffTime)

			ranksCacheMu.Lock()
			ranksCache[xuid] = entry
			ranksCacheMu.Unlock()
			return
		}

		// If not in cache and API fails, use default rank
		h.SetRanks([]rank.Rank{rank.Trainer})
		return
	}

	// API request successful, set ranks from roles
	ranks := rank.RolesToRanks(roles)
	h.SetRanks(ranks)

	// Update cache with freshly fetched ranks
	ranksCacheMu.Lock()
	ranksCache[xuid] = rankCacheEntry{
		ranks:    ranks,
		expiry:   time.Now().Add(ranksCacheTTL),
		attempts: 0,
	}
	ranksCacheMu.Unlock()
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

	// Load ranks again
	h.loadRanks(p.XUID())

	// Update player's name tag to reflect new rank
	p.SetNameTag(h.HighestRank().NameTag(p.Name()))
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

	for _, playerRank := range h.ranks {
		if playerRank == r {
			return true
		}
	}
	return false
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
