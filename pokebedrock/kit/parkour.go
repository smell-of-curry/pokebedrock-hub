package kit

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/sandertv/gophertunnel/minecraft/text"
)

// Parkour ...
var Parkour parkour

// parkour ...
type parkour struct{}

// Items ...
func (parkour) Items(*player.Player) (items [inventorySlots]item.Stack) {
	return [inventorySlots]item.Stack{
		0: item.NewStack(item.Spyglass{}, 1).
			WithCustomName(text.Colourf("<yellow>Toggle Players</yellow>")).
			WithValue("lobby", "toggle-visibility"),
		7: item.NewStack(item.FireCharge{}, 1).
			WithCustomName(text.Colourf("<red>Quit Parkour</red>")).
			WithValue("parkour", "quit"),
		8: item.NewStack(item.Feather{}, 1).
			WithCustomName(text.Colourf("<aqua>Restart</aqua>")).
			WithValue("parkour", "restart"),
	}
}

// ApplyFunc ...
func (parkour) ApplyFunc(p *player.Player) {
	_ = p.SetHeldSlot(0)
}
