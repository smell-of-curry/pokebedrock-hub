package rank

// rolesToRanks ...
var rolesToRanks = map[string]Rank{
	"1055833987739824258": Owner,
	"1067977172339396698": Manager,
	"1083171563798540349": Admin,
	"1131819233874022552": HeadModerator,
	"1295545506646462504": SeniorModerator,
	"1083171623282163743": Moderator,
	"1085669297034117200": HeadModeler,
	"1080719745290088498": Modeler,
	"1088902437093523566": Helper,
	"1281044331121217538": MonthlyTournamentMVP,
	"1084485790605787156": ContentCreator,
	"1096068279786815558": Premium,
	"1088998497061175296": Supporter,
	"1068578576951160952": ServerBooster,
	"1068581342159306782": Trainer,
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
