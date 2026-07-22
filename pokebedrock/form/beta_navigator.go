package form

import (
	"sort"
	"strconv"
	"strings"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/form"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/devserver"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

// betaNavigator lists dynamically registered dev-/beta servers.
type betaNavigator struct{}

// NewBetaNavigator ...
func NewBetaNavigator() form.Menu {
	f := form.NewMenu(betaNavigator{}, text.Colourf("Beta Navigator"))

	servers := devServers()
	if len(servers) == 0 {
		return f.WithBody(text.Colourf("<grey>No dev servers online.</grey>")).
			WithButtons(form.NewButton(text.Colourf("<grey>None available</grey>"), ""))
	}

	btns := make([]form.Button, 0, len(servers))
	for _, s := range servers {
		st := s.Status()

		var statusName string
		if st.Online {
			statusName = "<green>Online</green>"
		} else {
			statusName = "<dark-red>Offline</dark-red>"
		}

		name := text.Colourf("%s\n%s (%d<b>/</b>%d)", s.Name(), statusName, st.PlayerCount, st.MaxPlayerCount)
		btns = append(btns, form.NewButton(name, s.Icon()))
	}

	return f.WithButtons(btns...)
}

// Submit ...
func (betaNavigator) Submit(sub form.Submitter, b form.Button, _ *world.Tx) {
	p := sub.(*player.Player)
	serverName := text.Clean(strings.Split(b.Text, "\n")[0])
	server := srv.FromName(serverName)

	if server == nil || !devserver.IsDevIdentifier(server.Identifier()) {
		p.Message(text.Colourf("<dark-red>Failed to find server with name %s.</dark-red>", serverName))
		return
	}

	p.SendForm(NewServerConfirm(server))
}

func devServers() []*srv.Server {
	var out []*srv.Server
	for _, s := range srv.All() {
		if devserver.IsDevIdentifier(s.Identifier()) {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ii, jj := out[i].Identifier(), out[j].Identifier()
		if ii == "dev-beta" {
			return true
		}
		if jj == "dev-beta" {
			return false
		}
		return prNumber(ii) < prNumber(jj)
	})
	return out
}

func prNumber(id string) int {
	const prefix = "dev-pr-"
	if !strings.HasPrefix(id, prefix) {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimPrefix(id, prefix))
	return n
}
