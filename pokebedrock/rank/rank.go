package rank

import "github.com/sandertv/gophertunnel/minecraft/text"

// Rank ...
type Rank int

const (
	Trainer Rank = iota
	Premium
	Sponsor
	Moderator
	Admin
	Manager
	Owner
)

// Name ...
func (r Rank) Name() string {
	switch r {
	case Trainer:
		return "Trainer"
	case Premium:
		return "Premium"
	case Sponsor:
		return "Sponsor"
	case Moderator:
		return "Moderator"
	case Admin:
		return "Admin"
	case Manager:
		return "Manager"
	case Owner:
		return "Owner"
	}
	return "Unknown"
}

// formatName formats a player name according to their rank
func (r Rank) formatName(name string) string {
	switch r {
	case Trainer:
		return text.Colourf("<grey>%s</grey>", name)
	case Premium:
		return text.Colourf("<green>Premium %s</green>", name)
	case Sponsor:
		return text.Colourf("<gold>Sponsor %s</gold>", name)
	case Moderator:
		return text.Colourf("<blue>Moderator %s</blue>", name)
	case Admin:
		return text.Colourf("<red>Admin %s</red>", name)
	case Manager:
		return text.Colourf("<purple>Manager %s</purple>", name)
	case Owner:
		return text.Colourf("<dark-red>Owner %s</dark-red>", name)
	default:
		return text.Colourf("<grey>%s</grey>", name)
	}
}

// Chat ...
func (r Rank) Chat(name, message string) string {
	return text.Colourf("%s: <grey>%s</grey>", r.formatName(name), message)
}

// NameTag ...
func (r Rank) NameTag(name string) string {
	return r.formatName(name)
}
