package form

import (
	"strings"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/form"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

// ServerNavigator ...
type serverNavigator struct{}

// NewServerNavigator ...
func NewServerNavigator() form.Menu {
	f := form.NewMenu(serverNavigator{},
		text.Colourf("Server Navigator"))

	btns := make([]form.Button, 0, len(srv.All()))

	for _, s := range srv.All() {
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
func (serverNavigator) Submit(sub form.Submitter, b form.Button, _ *world.Tx) {
	p := sub.(*player.Player)
	serverName := text.Clean(strings.Split(b.Text, "\n")[0])
	server := srv.FromName(serverName)

	if server == nil {
		p.Message(text.Colourf("<dark-red>Failed to find server with name %s.</dark-red>", serverName))

		return
	}

	p.SendForm(NewServerConfirm(server))
}
