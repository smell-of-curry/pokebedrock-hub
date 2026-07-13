package npc

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player/skin"
	"github.com/go-gl/mathgl/mgl64"
)

// Settings holds initial NPC values. Runtime values may be changed on the
// returned *player.Player after Create.
type Settings struct {
	Name string
	Skin skin.Skin

	Position   mgl64.Vec3
	Yaw, Pitch float64
	Scale      float64
	Immobile   bool
	Vulnerable bool
	MainHand   item.Stack
	OffHand    item.Stack
	Helmet     item.Stack
	Chestplate item.Stack
	Leggings   item.Stack
	Boots      item.Stack
}
