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
		text.Colourf("<purple>Server Navigator</purple>")).
		WithBody("Pick a server to join:")

	var btns []form.Button
	for _, s := range srv.All() {
		st := s.Status()
		name := fmt.Sprintf("%s - [%d]", s.Name(), st.PlayerCount)
		btns = append(btns, form.NewButton(name, ""))
		btns = append(btns, form.NewButton(name, s.Icon()))
	}

	return f.WithButtons(btns...)
}

// Submit ...
func (serverNavigator) Submit(sub form.Submitter, b form.Button, _ *world.Tx) {
	name := strings.Split(text.Clean(b.Text), " -")[0]
	s := srv.FromName(name)
	if s == nil {
		return
	}
	sub.(*player.Player).SendForm(NewServerConfirm(s))
}
