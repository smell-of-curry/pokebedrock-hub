package identity

import "time"

// PlayerIdentity ...
type PlayerIdentity struct {
	ThirdPartyName string    `json:"name"`
	XUID           string    `json:"xuid"`
	Expiration     time.Time `json:"expiration"`
}
