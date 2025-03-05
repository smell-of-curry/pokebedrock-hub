package handler

import (
	"sync"
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/chat"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/form"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/kit"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/moderation"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// PlayerHandler ...
type PlayerHandler struct { // TODO: Move ranks & punishments within a session package.
	rankMu sync.Mutex
	ranks  []rank.Rank

	muted           atomic.Bool
	lastRankRefetch atomic.Value[time.Time]

	player.NopHandler
}

// NewPlayerHandler ...
func NewPlayerHandler(p *player.Player) *PlayerHandler {
	h := &PlayerHandler{}
	h.lastRankRefetch.Store(time.Time{})

	h.loadMute(p)

	// Display initial welcome message
	h.displayWelcomeMessage(p)

	// Queue async loading of ranks after player is fully initialized
	// This will update the player's nametag when ranks are loaded
	go func() {
		// Small delay to ensure player is fully initialized
		time.Sleep(100 * time.Millisecond)
		h.LoadRanksAsync(p.XUID(), p.H())
	}()

	return h
}

// loadMute ...
func (h *PlayerHandler) loadMute(p *player.Player) {
	resp, err := moderation.GlobalService().InflictionOfPlayer(p)
	if err != nil {
		h.muted.Store(false)
		return
	}

	h.muted.Store(false)
	for _, i := range resp.CurrentInflictions {
		if i.Type == moderation.InflictionMuted {
			h.muted.Store(true)
			break
		}
	}
}

// displayWelcomeMessage shows the appropriate welcome message based on current rank
func (h *PlayerHandler) displayWelcomeMessage(p *player.Player) {
	p.Message("Welcome to the PokeBedrock Hub!")
	p.Message("If you have priority queue, or want to sync your rank, ensure your discord is linked.")
	p.Message("Use /link in the Discord to link your roles.")
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
		case 1:
			w := p.Tx().World()
			p.Teleport(w.Spawn().Vec3Middle())
		case 2:
			// Check cooldown for rank refetch
			lastRefetch := h.lastRankRefetch.Load()
			if time.Since(lastRefetch) < time.Second*5 {
				remaining := time.Second*5 - time.Since(lastRefetch)
				p.SendTip(text.Colourf("<yellow>Please wait %.1f seconds before refreshing your rank again.</yellow>", remaining.Seconds()))
				return
			}

			h.lastRankRefetch.Store(time.Now())
			p.SendTip("Fetching your rank...")
			go func() {
				h.LoadRanksAsync(p.XUID(), p.H())
			}()
		}
	}
}

// HandleChat ...
func (h *PlayerHandler) HandleChat(ctx *player.Context, message *string) {
	ctx.Cancel()
	p := ctx.Val()

	if h.muted.Load() {
		p.Message(text.Colourf("<red>You're muted.</red>"))
		return
	}

	// Only allow users with ranks other than Trainer to chat
	if h.HighestRank() == rank.Trainer {
		p.Message(text.Colourf("<red>You must have a rank greater than Trainer to chat.</red>"))
		return
	}

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
