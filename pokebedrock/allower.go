package pokebedrock

import (
	"net"

	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/moderation"
)

// Allower ...
type Allower struct{}

// Allow ...
func (Allower) Allow(_ net.Addr, d login.IdentityData, _ login.ClientData) (string, bool) {
	resp, err := moderation.GlobalService().InflictionOfXUID(d.XUID)
	if err != nil {
		return locale.Translate("error.inflictions.load"), false
	}
	for _, i := range resp.CurrentInflictions {
		if i.Type == moderation.InflictionBanned {
			return locale.Translate("error.ban.message", i.Reason, i.ExpiryDate, i.Prosecutor), false
		}
	}
	return "", true
}
