package handler

import (
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/text"
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

	// Initialize with initial rank from cache only
	h.loadRanks(p.XUID())

	// Display initial welcome message
	h.displayWelcomeMessage(p)

	// Queue async loading of ranks after player is fully initialized
	// This will update the player's nametag when ranks are loaded
	go func() {
		// Small delay to ensure player is fully initialized
		time.Sleep(100 * time.Millisecond)
		h.LoadRanksAsync(p.XUID(), p)
	}()

	return h
}

// displayWelcomeMessage shows the appropriate welcome message based on current rank
func (h *PlayerHandler) displayWelcomeMessage(p *player.Player) {
	highestRank := h.HighestRank()
	if highestRank == rank.Trainer {
		// Player probably has not connected their discord account
		p.Message("Welcome to the PokeBedrock Hub! Your current rank is a Trainer.")
		p.Message("If you have priority queue, or want to sync your rank, ensure your discord is linked.")
		p.Message("Use /link in the Discord to link your roles.")
	} else {
		p.Messagef("Welcome %s, you have synced role: %s", p.Name(), highestRank.Name())
	}
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

	// TODO - Re-enable once moderation system is fixed.
	p.Message(text.Colourf("<red>Chat is currently disabled.</red>"))
	// msg := h.HighestRank().Chat(p.Name(), *message)
	// _, _ = chat.Global.WriteString(msg)
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
