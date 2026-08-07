// Package moderation provides moderation actions for the server.
package moderation

import "time"

// InflictionType represents the type of infliction that a player may face (e.g., banned, muted, etc.).
type InflictionType string

// InflictionType constants for the moderation service.
const (
	InflictionBanned InflictionType = "BANNED"
	InflictionMuted  InflictionType = "MUTED"
	InflictionFrozen InflictionType = "FROZEN"
	InflictionWarned InflictionType = "WARNED"
	InflictionKicked InflictionType = "KICKED"
)

// Infliction represents a player's moderation infliction (e.g., ban, mute, kick).
// DateInflicted and ExpiryDate are stored as unix milliseconds for internal use.
type Infliction struct {
	ID            string
	Type          InflictionType
	DateInflicted int64
	ExpiryDate    *int64
	Reason        string
	Prosecutor    string
}

// ModelResponse represents the response containing infliction data for a player.
// It separates the inflictions into current (active) and past (inactive) inflictions.
type ModelResponse struct {
	CurrentInflictions []Infliction
	PastInflictions    []Infliction
}

// UserContext identifies a player for API calls to the players service.
type UserContext struct {
	XUID      string `json:"xuid,omitempty"`
	Name      string `json:"name,omitempty"`
	DiscordID string `json:"discordId,omitempty"`
	IP        string `json:"ip,omitempty"`
}

// PlayerDetails represents the basic details of a player, including their name, XUID, and IP address.
type PlayerDetails struct {
	Name string `json:"name"`
	XUID string `json:"xuid"`
	IP   string `json:"ip"`
}

// ---------------------------------------------------------------------------
// Players service API types (JSON wire format)
// ---------------------------------------------------------------------------

// apiInflictionsResponse is the JSON shape returned by GET /api/players/inflictions.
type apiInflictionsResponse struct {
	Active   []apiInfliction `json:"active"`
	Inactive []apiInfliction `json:"inactive"`
}

// apiInfliction is a single infliction as returned by the players service.
type apiInfliction struct {
	ID             string  `json:"id"`
	Type           string  `json:"type"`
	DateInflicted  string  `json:"dateInflicted"`
	ExpiryDate     *string `json:"expiryDate"`
	Active         bool    `json:"active"`
	Reason         *string `json:"reason"`
	ProsecutorName *string `json:"prosecutorName"`
}

// apiCreateRequest is the JSON body for POST /api/inflictions.
type apiCreateRequest struct {
	UserContext UserContext         `json:"userContext"`
	Infliction apiInflictionCreate `json:"infliction"`
}

// apiInflictionCreate holds the infliction fields sent when creating a new infliction.
type apiInflictionCreate struct {
	Type           string  `json:"type"`
	Reason         string  `json:"reason,omitempty"`
	ProsecutorName string  `json:"prosecutorName,omitempty"`
	DateInflicted  string  `json:"dateInflicted,omitempty"`
	ExpiryDate     *string `json:"expiryDate,omitempty"`
}

// apiUpsertPlayer is the JSON body for POST /api/players (upsert).
type apiUpsertPlayer struct {
	XUID string   `json:"xuid"`
	Name string   `json:"name"`
	IPs  []string `json:"ips,omitempty"`
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

// apiInflictionToInternal converts a players-service infliction to the internal format.
func apiInflictionToInternal(a apiInfliction) Infliction {
	var dateInflicted int64
	if t, err := time.Parse(time.RFC3339Nano, a.DateInflicted); err == nil {
		dateInflicted = t.UnixMilli()
	}

	var expiryDate *int64
	if a.ExpiryDate != nil {
		if t, err := time.Parse(time.RFC3339Nano, *a.ExpiryDate); err == nil {
			ms := t.UnixMilli()
			expiryDate = &ms
		}
	}

	reason := ""
	if a.Reason != nil {
		reason = *a.Reason
	}

	prosecutor := ""
	if a.ProsecutorName != nil {
		prosecutor = *a.ProsecutorName
	}

	return Infliction{
		ID:            a.ID,
		Type:          InflictionType(a.Type),
		DateInflicted: dateInflicted,
		ExpiryDate:    expiryDate,
		Reason:        reason,
		Prosecutor:    prosecutor,
	}
}

// internalToAPICreate converts an internal Infliction to the players-service create payload.
func internalToAPICreate(i Infliction) apiInflictionCreate {
	a := apiInflictionCreate{
		Type:           string(i.Type),
		Reason:         i.Reason,
		ProsecutorName: i.Prosecutor,
	}
	if i.DateInflicted > 0 {
		a.DateInflicted = time.UnixMilli(i.DateInflicted).UTC().Format(time.RFC3339Nano)
	}
	if i.ExpiryDate != nil {
		s := time.UnixMilli(*i.ExpiryDate).UTC().Format(time.RFC3339Nano)
		a.ExpiryDate = &s
	}
	return a
}
