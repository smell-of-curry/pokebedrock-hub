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

// Config holds the configurable role IDs for ranks
type Config struct {
	TrainerRoleID              string
	ServerBoosterRoleID        string
	SupporterRoleID            string
	PremiumRoleID              string
	ContentCreatorRoleID       string
	MonthlyTournamentMVPRoleID string
	RetiredStaffRoleID         string
	HelperRoleID               string
	TeamRoleID                 string
	TranslatorRoleID           string
	DevelopmentTeamRoleID      string
	TrailModelerRoleID         string
	ModelerRoleID              string
	HeadModelerRoleID          string
	ModeratorRoleID            string
	SeniorModeratorRoleID      string
	HeadModeratorRoleID        string
	AdminRoleID                string
	ManagerRoleID              string
	OwnerRoleID                string
}

// rankInfos holds the rank details in the same order as the Rank constants.
var rankInfos map[Rank]Info

// InitializeRanks initializes the rank system with the provided configuration
func InitializeRanks(config *Config) {
	rankInfos = map[Rank]Info{
		UnLinked: {DisplayName: "UnLinked", Color: "grey", Prefix: false},
		Trainer:  {DisplayName: "Trainer", Color: "white", Prefix: true, RoleID: config.TrainerRoleID},
		ServerBooster: {DisplayName: "Server Booster", Color: "diamond", Prefix: true,
			RoleID: config.ServerBoosterRoleID},
		Supporter: {DisplayName: "Supporter", Color: "emerald", Prefix: true, RoleID: config.SupporterRoleID},
		Premium:   {DisplayName: "Premium", Color: "green", Prefix: true, RoleID: config.PremiumRoleID},
		ContentCreator: {DisplayName: "Content Creator", Color: "amethyst", Prefix: true,
			RoleID: config.ContentCreatorRoleID},
		MonthlyTournamentMVP: {DisplayName: "Monthly Tournament MVP", Color: "aqua", Prefix: true,
			RoleID: config.MonthlyTournamentMVPRoleID},
		RetiredStaff: {DisplayName: "Retired Staff", Color: "grey", Prefix: true,
			RoleID: config.RetiredStaffRoleID},
		Helper: {DisplayName: "Helper", Color: "yellow", Prefix: true, RoleID: config.HelperRoleID},
		Team:   {DisplayName: "Team", Color: "gold", Prefix: true, RoleID: config.TeamRoleID},
		Translator: {DisplayName: "Translator", Color: "dark-yellow", Prefix: true,
			RoleID: config.TranslatorRoleID},
		DevelopmentTeam: {DisplayName: "Development Team", Color: "redstone", Prefix: true,
			RoleID: config.DevelopmentTeamRoleID},
		TrailModeler: {DisplayName: "Trail Modeler", Color: "dark-green", Prefix: true,
			RoleID: config.TrailModelerRoleID},
		Modeler: {DisplayName: "Modeler", Color: "purple", Prefix: true, RoleID: config.ModelerRoleID},
		HeadModeler: {DisplayName: "Head Modeler", Color: "dark-purple", Prefix: true,
			RoleID: config.HeadModelerRoleID},
		Moderator: {DisplayName: "Moderator", Color: "blue", Prefix: true, RoleID: config.ModeratorRoleID},
		SeniorModerator: {DisplayName: "Senior Moderator", Color: "aqua", Prefix: true,
			RoleID: config.SeniorModeratorRoleID},
		HeadModerator: {DisplayName: "Head Moderator", Color: "dark-blue", Prefix: true,
			RoleID: config.HeadModeratorRoleID},
		Admin:   {DisplayName: "Admin", Color: "red", Prefix: true, RoleID: config.AdminRoleID},
		Manager: {DisplayName: "Manager", Color: "purple", Prefix: true, RoleID: config.ManagerRoleID},
		Owner:   {DisplayName: "Owner", Color: "dark-red", Prefix: true, RoleID: config.OwnerRoleID},
	}

	// Rebuild the role to rank mapping
	rolesToRanks = make(map[string]Rank)

	for r, info := range rankInfos {
		if info.RoleID != "" {
			rolesToRanks[info.RoleID] = r
		}
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
