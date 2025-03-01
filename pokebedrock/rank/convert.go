package rank

// rolesToRanks ...
var rolesToRanks = map[string]Rank{
	"1068581342159306782": Trainer,
	"1096068279786815558": Premium,
	"1088998497061175296": Sponsor,
	"1083171623282163743": Moderator,
	"1083171563798540349": Admin,
	"1067977172339396698": Manager,
	"1055833987739824258": Owner,
}

// RolesToRanks ...
func RolesToRanks(roles []string) []Rank {
	var ranks []Rank
	for _, role := range roles {
		if rank, ok := rolesToRanks[role]; ok {
			ranks = append(ranks, rank)
		}
	}
	return ranks
}
