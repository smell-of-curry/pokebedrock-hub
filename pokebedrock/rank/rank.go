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

// Chat ...
func (r Rank) Chat(name, message string) string {
	switch r { // TODO: Adjust for the rest.
	case Admin:
		return text.Colourf("<red>Admin %s</red>: <grey>%s</grey>", name, message)
	case Owner:
		return text.Colourf("<dark-red>Owner %s</dark-red>: <grey>%s</grey>", name, message)
	}
	return text.Colourf("<grey>%s: %s</grey>", name, message)
}

// NameTag ...
func (r Rank) NameTag(name string) string {
	switch r { // TODO: Adjust for the rest.
	case Admin:
		return text.Colourf("<red>Admin %s</red>", name)
	case Owner:
		return text.Colourf("<dark-red>Owner %s</dark-red>", name)
	}
	return text.Colourf("<grey>%s</grey>", name)
}
