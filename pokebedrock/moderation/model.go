package moderation

// InflictionType ...
type InflictionType string

const (
	InflictionBanned InflictionType = "BANNED"
	InflictionMuted  InflictionType = "MUTED"
	InflictionFrozen InflictionType = "FROZEN"
	InflictionWarned InflictionType = "WARNED"
	InflictionKicked InflictionType = "KICKED"
)

// InflictionStatus ...
type InflictionStatus string

const (
	InflictionStatusCurrent = "current"
	InflictionStatusPast    = "past"
)

// Infliction ...
type Infliction struct {
	Type          InflictionType `json:"type"`
	DateInflicted int64          `json:"date_inflicted"`
	ExpiryDate    *int64         `json:"expiry_date,omitempty"`
	Reason        string         `json:"reason"`
	Prosecutor    string         `json:"prosecutor"`
}

// ModelRequest ...
type ModelRequest struct {
	XUID             string           `json:"xuid,omitempty"`
	Name             string           `json:"name,omitempty"`
	DiscordID        string           `json:"discord_id,omitempty"`
	IP               string           `json:"ip,omitempty"`
	InflictionStatus InflictionStatus `json:"inflictionStatus"`
	Infliction       Infliction       `json:"infliction"`
}

// ModelResponse ...
type ModelResponse struct {
	CurrentInflictions []Infliction `json:"current_inflictions"`
	PastInflictions    []Infliction `json:"past_inflictions"`
}
