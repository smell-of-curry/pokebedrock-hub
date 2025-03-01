package handler

import (
	"sort"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/data"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// loadRanks ...
func (h *PlayerHandler) loadRanks(xuid string) {
	roles, err := data.Roles(xuid)
	if err != nil {
		h.SetRanks([]rank.Rank{rank.Trainer})
		return
	}
	h.SetRanks(rank.RolesToRanks(roles))
}

// SetRanks ...
func (h *PlayerHandler) SetRanks(ranks []rank.Rank) {
	h.rankMu.Lock()
	h.ranks = ranks
	h.rankMu.Unlock()
	h.sortRanks()
}

// HighestRank ...
func (h *PlayerHandler) HighestRank() rank.Rank {
	h.rankMu.Lock()
	defer h.rankMu.Unlock()
	return h.ranks[len(h.ranks)-1]
}

// Ranks ...
func (h *PlayerHandler) Ranks() []rank.Rank {
	h.rankMu.Lock()
	defer h.rankMu.Unlock()
	return h.ranks
}

// sortRanks ...
func (h *PlayerHandler) sortRanks() {
	sort.SliceStable(h.ranks, func(i, j int) bool {
		return h.ranks[i] < h.ranks[j]
	})
}
