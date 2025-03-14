package pokebedrock

import (
	"log/slog"
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
		// TODO: Change this back to disabled once fixed.
		slog.Default().Error("error whilst loading inflictions", "xuid", d.XUID, "error", err)
		return locale.Translate("error.inflictions.load"), true
	}
	for _, i := range resp.CurrentInflictions {
		if i.Type == moderation.InflictionBanned {
			return locale.Translate("error.ban.message", i.Reason, i.ExpiryDate, i.Prosecutor), false
		}
	}
	return "", true
}
