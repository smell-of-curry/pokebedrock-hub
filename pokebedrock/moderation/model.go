package moderation

// InflictionType ...
type InflictionType string

const (
	Banned InflictionType = "BANNED"
	Muted  InflictionType = "MUTED"
	Frozen InflictionType = "FROZEN"
	Warned InflictionType = "WARNED"
	Kicked InflictionType = "KICKED"
)

// Infliction ...
type Infliction struct {
	Type          InflictionType `json:"type"`
	DateInflicted int64          `json:"date_inflicted"`
	ExpiryDate    int64          `json:"expiry_date,omitempty"`
	Reason        string         `json:"reason"`
	Prosecutor    string         `json:"prosecutor"`
}

// ModelRequest ...
type ModelRequest struct {
	XUID             string     `json:"xuid,omitempty"`
	Name             string     `json:"name,omitempty"`
	DiscordID        string     `json:"discord_id,omitempty"`
	IP               string     `json:"ip,omitempty"`
	InflictionStatus string     `json:"inflictionStatus"`
	Infliction       Infliction `json:"infliction"`
}

// ModelResponse ...
type ModelResponse struct {
	CurrentInflictions []Infliction `json:"current_inflictions"`
	PastInflictions    []Infliction `json:"past_inflictions"`
}
