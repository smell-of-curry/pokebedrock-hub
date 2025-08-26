// Package rank provides a conversion between external role IDs and in-game ranks.
package rank

import "sort"

// rolesToRanks maps external role IDs to in-game ranks.
// It is built automatically from rankInfos.
var rolesToRanks = make(map[string]Rank)

func init() {
	for r, info := range rankInfos {
		if info.RoleID != "" {
			rolesToRanks[info.RoleID] = r
		}
	}
}

// RolesToRanks converts a slice of external role IDs into a slice of in-game ranks.
func RolesToRanks(roles []string) []Rank {
	var ranks []Rank

	for _, role := range roles {
		if rank, ok := rolesToRanks[role]; ok {
			ranks = append(ranks, rank)
		}
	}

	return ranks
}

// GetHighestRank returns the highest rank of a player based on their roles.
func GetHighestRank(roles []string) Rank {
	ranks := RolesToRanks(roles)

	if len(ranks) == 0 {
		return UnLinked
	}

	sort.SliceStable(ranks, func(i, j int) bool {
		return ranks[i] < ranks[j]
	})

	return ranks[len(ranks)-1]
}
