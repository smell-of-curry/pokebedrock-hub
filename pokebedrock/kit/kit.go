package kit

import (
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
)

// Kit ...
type Kit interface {
	Items(*player.Player) (items [36]item.Stack)
	ApplyFunc(*player.Player)
}

// Apply ...
func Apply(k Kit, p *player.Player) {
	p.Inventory().Clear()
	p.Armour().Clear()

	p.SetHeldItems(item.Stack{}, item.Stack{})

	p.StopSneaking()
	p.StopSwimming()
	p.StopSprinting()
	p.StopFlying()
	p.ResetFallDistance()
	p.SetGameMode(world.GameModeAdventure)

	p.Heal(20, entity.FoodHealingSource{})
	p.SetFood(20)
	for _, eff := range p.Effects() {
		p.RemoveEffect(eff.Type())
	}

	inv := p.Inventory()
	for slot, it := range k.Items(p) {
		_ = inv.SetItem(slot, it)
	}

	k.ApplyFunc(p)
}
