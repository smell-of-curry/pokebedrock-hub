package rank

import "github.com/sandertv/gophertunnel/minecraft/text"

// Rank represents the rank of a player.
type Rank int

// Rank constants for the rank service.
const (
	UnLinked Rank = iota
	Trainer
	ServerBooster
	Supporter
	Premium
	ContentCreator
	MonthlyTournamentMVP
	RetiredStaff
	Helper
	Team
	Translator
	DevelopmentTeam
	TrailModeler
	Modeler
	HeadModeler
	Moderator
	SeniorModerator
	HeadModerator
	Admin
	Manager
	Owner
)

// Info centralizes all details for each rank.
type Info struct {
	DisplayName string // Human-readable name of the rank.
	Color       string // Color to be used (must follow the color guidelines).
	Prefix      bool   // If true, the rank's title is prepended to the player's name.
	RoleID      string // External role identifier.
}

// rankInfos holds the rank details in the same order as the Rank constants.
var rankInfos = map[Rank]Info{
	UnLinked:             {DisplayName: "UnLinked", Color: "grey", Prefix: false},
	Trainer:              {DisplayName: "Trainer", Color: "white", Prefix: true, RoleID: "1068581342159306782"},
	ServerBooster:        {DisplayName: "Server Booster", Color: "diamond", Prefix: true, RoleID: "1068578576951160952"},
	Supporter:            {DisplayName: "Supporter", Color: "emerald", Prefix: true, RoleID: "1088998497061175296"},
	Premium:              {DisplayName: "Premium", Color: "green", Prefix: true, RoleID: "1096068279786815558"},
	ContentCreator:       {DisplayName: "Content Creator", Color: "amethyst", Prefix: true, RoleID: "1084485790605787156"},
	MonthlyTournamentMVP: {DisplayName: "Monthly Tournament MVP", Color: "aqua", Prefix: true, RoleID: "1281044331121217538"},
	RetiredStaff:         {DisplayName: "Retired Staff", Color: "grey", Prefix: true, RoleID: "1179937172455952384"},
	Helper:               {DisplayName: "Helper", Color: "yellow", Prefix: true, RoleID: "1088902437093523566"},
	Team:                 {DisplayName: "Team", Color: "gold", Prefix: true, RoleID: "1067977855700574238"},
	Translator:           {DisplayName: "Translator", Color: "dark-yellow", Prefix: true, RoleID: "1137751922217058365"},
	DevelopmentTeam:      {DisplayName: "Development Team", Color: "redstone", Prefix: true, RoleID: "1123082881380646944"},
	TrailModeler:         {DisplayName: "Trail Modeler", Color: "dark-green", Prefix: true, RoleID: "1085669665298194482"},
	Modeler:              {DisplayName: "Modeler", Color: "purple", Prefix: true, RoleID: "1080719745290088498"},
	HeadModeler:          {DisplayName: "Head Modeler", Color: "dark-purple", Prefix: true, RoleID: "1085669297034117200"},
	Moderator:            {DisplayName: "Moderator", Color: "blue", Prefix: true, RoleID: "1083171623282163743"},
	SeniorModerator:      {DisplayName: "Senior Moderator", Color: "aqua", Prefix: true, RoleID: "1295545506646462504"},
	HeadModerator:        {DisplayName: "Head Moderator", Color: "dark-blue", Prefix: true, RoleID: "1131819233874022552"},
	Admin:                {DisplayName: "Admin", Color: "red", Prefix: true, RoleID: "1083171563798540349"},
	Manager:              {DisplayName: "Manager", Color: "purple", Prefix: true, RoleID: "1067977172339396698"},
	Owner:                {DisplayName: "Owner", Color: "dark-red", Prefix: true, RoleID: "1055833987739824258"},
}

func init() {
	for r, info := range rankInfos {
		rolesToRanks[info.RoleID] = r
	}
}

// Name returns the human-readable name of the rank.
func (r Rank) Name() string {
	if int(r) < 0 || int(r) >= len(rankInfos) {
		return "Unknown"
	}

	return rankInfos[r].DisplayName
}

// FormatName formats a player's name according to their rank.
// If the rank uses a prefix, the rank's title is prepended.
func (r Rank) FormatName(name string) string {
	if _, ok := rankInfos[r]; !ok {
		return text.Colourf("<grey>%s</grey>", name)
	}

	info := rankInfos[r]
	if info.Prefix {
		return text.Colourf("<%s>%s %s</%s>", info.Color, info.DisplayName, name, info.Color)
	}

	return text.Colourf("<%s>%s</%s>", info.Color, name, info.Color)
}

// Chat formats a chat message with the rank's styled name.
func (r Rank) Chat(name, message string) string {
	return text.Colourf("%s: <grey>%s</grey>", r.FormatName(name), message)
}

// NameTag returns the formatted name tag of the player.
func (r Rank) NameTag(name string) string {
	return r.FormatName(name)
}
