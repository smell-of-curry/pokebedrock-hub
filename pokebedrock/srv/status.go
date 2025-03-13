package srv

// Status represents the current state of a server.
type Status struct {
	Online         bool
	PlayerCount    int
	MaxPlayerCount int
}
