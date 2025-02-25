package rank

import "github.com/sandertv/gophertunnel/minecraft/text"

// Rank ...
type Rank int

const (
	Vip Rank = iota
	Admin
	Owner
)

// Name ...
func (r Rank) Name() string {
	switch r {
	case Vip:
		return "VIP"
	case Admin:
		return "Admin"
	case Owner:
		return "Owner"
	}
	return "Unknown"
}

// Chat ...
func (r Rank) Chat(name, message string) string {
	switch r {
	case Vip:
		return text.Colourf("<grey>%s: %s</grey>", name, message)
	case Admin:
		return text.Colourf("<red>Admin %s</red>: <grey>%s</grey>", name, message)
	case Owner:
		return text.Colourf("<dark-red>Owner %s</dark-red>: <grey>%s</grey>", name, message)
	}
	return text.Colourf("<grey>%s: %s</grey>", name, message)
}

// NameTag ...
func (r Rank) NameTag(name string) string {
	switch r {
	case Vip:
		return text.Colourf("<grey>%s</grey>", name)
	case Admin:
		return text.Colourf("<red>Admin %s</red>", name)
	case Owner:
		return text.Colourf("<dark-red>Owner %s</dark-red>", name)
	}
	return text.Colourf("<grey>%s</grey>", name)
}
