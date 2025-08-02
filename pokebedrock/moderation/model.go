// Package moderation provides moderation actions for the server.
package moderation

// InflictionType represents the type of infliction that a player may face (e.g., banned, muted, etc.).
// It is used to categorize the infliction type in moderation actions.
type InflictionType string

// InflictionType constants for the moderation service.
const (
	InflictionBanned InflictionType = "BANNED"
	InflictionMuted  InflictionType = "MUTED"
	InflictionFrozen InflictionType = "FROZEN"
	InflictionWarned InflictionType = "WARNED"
	InflictionKicked InflictionType = "KICKED"
)

// InflictionStatus represents the status of an infliction, either current or past.
// It is used to differentiate between ongoing and historical inflictions.
type InflictionStatus string

// InflictionStatus constants for the moderation service.
const (
	InflictionStatusCurrent = "current"
	InflictionStatusPast    = "past"
)

// Infliction represents a player's moderation infliction (e.g., ban, mute, kick).
// It includes details such as the type of infliction, reason, and date information.
type Infliction struct {
	Type          InflictionType `json:"type"`
	DateInflicted int64          `json:"date_inflicted"`
	ExpiryDate    *int64         `json:"expiry_date,omitempty"`
	Reason        string         `json:"reason"`
	Prosecutor    string         `json:"prosecutor"`
}

// ModelRequest represents a request to fetch or interact with a player's infliction data.
// It contains player identifiers (e.g., XUID, name) and infliction-related status.
type ModelRequest struct {
	XUID             string           `json:"xuid,omitempty"`
	Name             string           `json:"name,omitempty"`
	DiscordID        string           `json:"discord_id,omitempty"`
	IP               string           `json:"ip,omitempty"`
	InflictionStatus InflictionStatus `json:"inflictionStatus"`
	Infliction       Infliction       `json:"infliction"`
}

// ModelResponse represents the response containing infliction data for a player.
// It separates the inflictions into current and past inflictions.
type ModelResponse struct {
	CurrentInflictions []Infliction `json:"current_inflictions"`
	PastInflictions    []Infliction `json:"past_inflictions"`
}

// PlayerDetails represents the basic details of a player, including their name, XUID, and IP address.
// This is typically used for interacting with the player in moderation-related tasks.
type PlayerDetails struct {
	Name string `json:"name"`
	XUID string `json:"xuid"`
	IP   string `json:"ip"`
}
