package pokebedrock

import (
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/protocol/login"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/moderation"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/vpn"
)

// Allower ...
type Allower struct{}

// Allow ...
func (a Allower) Allow(addr net.Addr, d login.IdentityData, _ login.ClientData) (string, bool) {
	if reason, allowed := a.handleVPN(addr, d); !allowed {
		return reason, allowed
	}

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

// handleVPN checks if the given network address is using a VPN and returns the reason and whether it's allowed.
// Accounts linked to the Discord server (holding at least one role) are allowed to connect through a VPN/proxy.
func (Allower) handleVPN(netAddr net.Addr, d login.IdentityData) (reason string, allowed bool) {
	addr, err := netip.ParseAddrPort(netAddr.String())
	if err != nil {
		slog.Default().Error("error whilst parsing address", "address", netAddr.String(), "error", err)

		return "Invalid address format.", false
	}

	ip := addr.Addr()
	if ip.IsLoopback() || ip.IsUnspecified() {
		return "", true
	}

	addrString := ip.String()

	m, err := vpn.GlobalService().CheckIP(addrString)
	if err != nil {
		// Allow players through when VPN service is rate limited
		if strings.Contains(err.Error(), "rate limit active") {
			slog.Default().Warn("VPN check skipped due to rate limit", "ip", addrString, "error", err)

			return "", true
		}

		return err.Error(), false
	}

	if m.Status != vpn.StatusSuccess {
		return m.Message, false
	}

	if !m.Proxy {
		return "", true
	}

	// Linked accounts (any Discord role) may connect through a VPN/proxy.
	roles, err := rank.GlobalService().RolesOfXUID(d.XUID)
	if err == nil && len(roles) > 0 {
		slog.Default().Info("allowing VPN connection for linked account",
			"ip", addrString, "name", d.DisplayName, "xuid", d.XUID)

		return "", true
	}

	if err != nil && !errors.Is(err, rank.ErrUserNotFound) {
		slog.Default().Error("error whilst checking roles for VPN bypass", "xuid", d.XUID, "error", err)
	}

	// Log denials with ISP details so misclassified residential ISP
	// ranges can be spotted and added to the VpnWhitelist config.
	slog.Default().Warn("blocked VPN/proxy connection",
		"ip", addrString, "isp", m.Isp, "org", m.Org, "name", d.DisplayName, "xuid", d.XUID)

	return locale.Translate("error.vpn.blocked"), false
}
