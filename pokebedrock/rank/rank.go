package rank

import "github.com/sandertv/gophertunnel/minecraft/text"

// Rank represents the rank of a player.
type Rank int

const (
	// Base ranks
	Trainer Rank = iota
	ServerBooster
	Supporter
	Premium
	ContentCreator
	MonthlyTournamentMVP
	Helper
	Modeler
	HeadModeler
	Moderator
	SeniorModerator
	HeadModerator
	Admin
	Manager
	Owner
)

// Name returns the human-readable name of the rank.
func (r Rank) Name() string {
	switch r {
	case Trainer:
		return "Trainer"
	case Premium:
		return "Premium"
	case Moderator:
		return "Moderator"
	case Admin:
		return "Admin"
	case Manager:
		return "Manager"
	case Owner:
		return "Owner"
	case HeadModerator:
		return "Head Moderator"
	case SeniorModerator:
		return "Senior Moderator"
	case HeadModeler:
		return "Head Modeler"
	case Modeler:
		return "Modeler"
	case Helper:
		return "Helper"
	case MonthlyTournamentMVP:
		return "Monthly Tournament MVP"
	case ContentCreator:
		return "Content Creator"
	case Supporter:
		return "Supporter"
	case ServerBooster:
		return "Server Booster"
	}
	return "Unknown"
}

// formatName formats a player name according to their rank.
func (r Rank) formatName(name string) string {
	switch r {
	case Trainer:
		return text.Colourf("<grey>%s</grey>", name)
	case Premium:
		return text.Colourf("<green>Premium %s</green>", name)
	case Moderator:
		return text.Colourf("<blue>Moderator %s</blue>", name)
	case Admin:
		return text.Colourf("<red>Admin %s</red>", name)
	case Manager:
		return text.Colourf("<purple>Manager %s</purple>", name)
	case Owner:
		return text.Colourf("<dark-red>Owner %s</dark-red>", name)
	case HeadModerator:
		return text.Colourf("<dark-blue>Head Moderator %s</dark-blue>", name)
	case SeniorModerator:
		return text.Colourf("<aqua>Senior Moderator %s</aqua>", name)
	case HeadModeler:
		return text.Colourf("<dark-purple>Head Modeler %s</dark-purple>", name)
	case Modeler:
		return text.Colourf("<purple>Modeler %s</purple>", name)
	case Helper:
		return text.Colourf("<yellow>Helper %s</yellow>", name)
	case MonthlyTournamentMVP:
		return text.Colourf("<aqua>Monthly Tournament MVP %s</aqua>", name)
	case ContentCreator:
		return text.Colourf("<pink>Content Creator %s</pink>", name)
	case Supporter:
		return text.Colourf("<light-gray>Supporter %s</light-gray>", name)
	case ServerBooster:
		return text.Colourf("<dark-green>Server Booster %s</dark-green>", name)
	default:
		return text.Colourf("<grey>%s</grey>", name)
	}
}

// Chat formats a chat message with the rank's name style.
func (r Rank) Chat(name, message string) string {
	return text.Colourf("%s: <grey>%s</grey>", r.formatName(name), message)
}

// NameTag returns the formatted name tag of the player.
func (r Rank) NameTag(name string) string {
	return r.formatName(name)
}
