package npc

import (
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// Create spawns a static player NPC in tx. Hub NPCs are immobile and never
// change worlds, so there is no 20 Hz world-sync goroutine.
//
// @param s Initial NPC settings.
// @param tx World transaction used to add the entity.
// @param f Optional interaction callback; nil is treated as a no-op.
// @returns the spawned NPC player entity.
func Create(s Settings, tx *world.Tx, f HandlerFunc) *player.Player {
	if tx == nil {
		panic("npc.Create: tx must not be nil")
	}
	if f == nil {
		f = func(*player.Player) {}
	}

	opts := world.EntitySpawnOpts{Position: s.Position}
	handle := opts.New(player.Type, player.Config{
		Name:     s.Name,
		Position: s.Position,
		Skin:     s.Skin,
	})
	loader := world.NewLoader(1, tx.World(), world.NopViewer{})

	ent := tx.AddEntity(handle)
	npc := ent.(*player.Player)

	npc.Move(mgl64.Vec3{}, s.Yaw, s.Pitch)
	npc.SetScale(s.Scale)
	npc.SetHeldItems(s.MainHand, s.OffHand)
	npc.Armour().Set(s.Helmet, s.Chestplate, s.Leggings, s.Boots)
	if s.Immobile {
		npc.SetImmobile()
	}

	h := &handler{f: f, l: loader, vulnerable: s.Vulnerable}
	npc.Handle(h)
	h.syncPosition(tx, s.Position)

	return npc
}
