package npc

import (
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// HandlerFunc handles a player interacting with an NPC.
type HandlerFunc func(p *player.Player)

// handler keeps the NPC's world.Loader aligned with its position and
// dispatches interaction callbacks.
type handler struct {
	player.NopHandler

	l          *world.Loader
	f          HandlerFunc
	vulnerable bool
}

// HandleHurt ...
func (h *handler) HandleHurt(ctx *player.Context, _ *float64, _ bool, _ *time.Duration, src world.DamageSource) {
	if attack, ok := src.(entity.AttackDamageSource); ok {
		if attacker, ok := attack.Attacker.(*player.Player); ok {
			h.f(attacker)
		}
	}
	if !h.vulnerable {
		ctx.Cancel()
	}
}

// HandleMove ...
func (h *handler) HandleMove(ctx *player.Context, pos mgl64.Vec3, _ cube.Rotation) {
	h.syncPosition(ctx.Player().Tx(), pos)
}

// HandleTeleport ...
func (h *handler) HandleTeleport(ctx *player.Context, pos mgl64.Vec3) {
	h.syncPosition(ctx.Player().Tx(), pos)
}

func (h *handler) syncPosition(tx *world.Tx, pos mgl64.Vec3) {
	h.l.Move(tx, pos)
	h.l.Load(tx, 1)
}

// HandleQuit ...
func (h *handler) HandleQuit(p *player.Player) {
	h.l.Close(p.Tx())
}
