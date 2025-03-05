package rank

// rolesToRanks maps external role IDs to in-game ranks.
// It is built automatically from rankInfos.
var rolesToRanks = make(map[string]Rank)

func init() {
	for i, info := range rankInfos {
		rolesToRanks[info.RoleID] = Rank(i)
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
