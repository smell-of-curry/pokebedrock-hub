package authentication

import "time"

// PlayerIdentity ...
type PlayerIdentity struct {
	DisplayName string    `json:"name"`
	XUID        string    `json:"xuid"`
	Expiration  time.Time `json:"expiration"`
}
