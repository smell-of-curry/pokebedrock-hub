package form

import (
	"fmt"
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
		text.Colourf("<purple>Server Navigator</purple>"))

	var btns []form.Button
	for _, s := range srv.All() {
		st := s.Status()
		statusName := "Unknown"
		if st.Online {
			statusName = "§aOnline"
		} else {
			statusName = "§4Offline"
		}

		name := fmt.Sprintf("%s\n%s§r (%d§l/§r%d)", s.Name(), statusName, st.PlayerCount, st.MaxPlayerCount)
		btns = append(btns, form.NewButton(name, s.Icon()))
	}

	return f.WithButtons(btns...)
}

// Submit ...
func (serverNavigator) Submit(sub form.Submitter, b form.Button, _ *world.Tx) {
	serverName := strings.Split(b.Text, "\n")[0]
	s := srv.FromName(serverName)
	if s == nil {
		sub.(*player.Player).Messagef("§4Failed to find server with name %s", serverName)
		return
	}
	sub.(*player.Player).SendForm(NewServerConfirm(s))
}
