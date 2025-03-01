package handler

import (
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/chat"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/form"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/kit"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// PlayerHandler ...
type PlayerHandler struct {
	rankMu sync.Mutex
	ranks  []rank.Rank

	player.NopHandler
}

// NewPlayerHandler ...
func NewPlayerHandler(p *player.Player) *PlayerHandler {
	h := &PlayerHandler{}
	h.loadRanks(p.XUID())
	return h
}

// HandleJoin ...
func (h *PlayerHandler) HandleJoin(p *player.Player, w *world.World) {
	p.Inventory().Handle(InventoryHandler{})
	p.Teleport(w.Spawn().Vec3Middle())
	p.SetNameTag(h.HighestRank().NameTag(p.Name()))

	kit.Apply(kit.Lobby, p)
}

// HandleItemUse ...
func (h *PlayerHandler) HandleItemUse(ctx *player.Context) {
	p := ctx.Val()
	it, _ := p.HeldItems()
	if id, ok := it.Value("lobby"); ok {
		switch id {
		case 0:
			p.SendForm(form.NewServerNavigator())
		}
	}
}

// HandleChat ...
func (h *PlayerHandler) HandleChat(ctx *player.Context, message *string) {
	ctx.Cancel()
	p := ctx.Val()

	msg := h.HighestRank().Chat(p.Name(), *message)
	_, _ = chat.Global.WriteString(msg)
}

// HandleFoodLoss ...
func (h *PlayerHandler) HandleFoodLoss(ctx *player.Context, _ int, _ *int) {
	ctx.Cancel()
}

// HandleBlockPlace ...
func (h *PlayerHandler) HandleBlockPlace(ctx *player.Context, _ cube.Pos, _ world.Block) {
	ctx.Cancel()
}

// HandleBlockBreak ...
func (h *PlayerHandler) HandleBlockBreak(ctx *player.Context, _ cube.Pos, _ *[]item.Stack, _ *int) {
	ctx.Cancel()
}

// HandleItemUseOnBlock ...
func (h *PlayerHandler) HandleItemUseOnBlock(ctx *player.Context, _ cube.Pos, _ cube.Face, _ mgl64.Vec3) {
	ctx.Cancel()
}

// HandleHurt ...
func (h *PlayerHandler) HandleHurt(ctx *player.Context, _ *float64, _ bool, _ *time.Duration, _ world.DamageSource) {
	ctx.Cancel()
}

// HandleItemDrop ...
func (h *PlayerHandler) HandleItemDrop(ctx *player.Context, _ item.Stack) {
	ctx.Cancel()
}

// HandleItemDamage ...
func (h *PlayerHandler) HandleItemDamage(ctx *player.Context, _ item.Stack, _ int) {
	ctx.Cancel()
}
