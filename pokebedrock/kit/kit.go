// Package kit provides kits for the server.
package kit

import (
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
)

// Kit defines the structure for a kit of items and actions that can be applied to a player.
// It includes methods to retrieve the items for the kit and apply additional functions to the player.
type Kit interface {
	Items(*player.Player) (items [36]item.Stack)
	ApplyFunc(*player.Player)
}

// Apply applies a kit to a player. It clears the player's inventory, armour, and other attributes,
// and then applies the items and effects defined in the provided kit.
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
