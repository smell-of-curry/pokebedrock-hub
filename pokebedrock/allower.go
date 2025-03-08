package pokebedrock

import (
	"net"

	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/moderation"
)

// Allower ...
type Allower struct{}

// Allow ...
func (Allower) Allow(_ net.Addr, d login.IdentityData, _ login.ClientData) (string, bool) {
	resp, err := moderation.GlobalService().InflictionOfXUID(d.XUID)
	if err != nil {
		return text.Colourf("<yellow>There was an error whilst loading your inflictions. Please try relogging and contact support if the issue persists.</yellow>"), false
	}
	for _, i := range resp.CurrentInflictions {
		if i.Type == moderation.InflictionBanned {
			return text.Colourf("<red>You're banned! Reason: %s, Expiry Date: %d, Prosecutor: %s</red>", i.Reason, i.ExpiryDate, i.Prosecutor), false
		}
	}
	return "", true
}
