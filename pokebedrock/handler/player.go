package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/chat"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/form"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/kit"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/session"
)

// PlayerHandler ...
type PlayerHandler struct {
	ranks       *session.Ranks
	inflictions *session.Inflictions

	player.NopHandler
}

// NewPlayerHandler ...
func NewPlayerHandler(p *player.Player) *PlayerHandler {
	h := &PlayerHandler{
		ranks:       session.NewRanks(),
		inflictions: session.NewInflictions(),
	}

	go h.inflictions.Load(p.H())
	go func() {
		// Small delay to ensure player is fully initialized
		time.Sleep(100 * time.Millisecond)
		h.Ranks().Load(p.XUID(), p.H())
	}()

	return h
}

// HandleJoin ...
func (h *PlayerHandler) HandleJoin(p *player.Player, w *world.World) {
	p.Inventory().Handle(InventoryHandler{})
	p.Teleport(w.Spawn().Vec3Middle())
	p.SetNameTag(h.Ranks().HighestRank().NameTag(p.Name()))

	kit.Apply(kit.Lobby, p)

	msg := locale.Translate("welcome.hub")
	for l := range strings.SplitSeq(msg, "<new-line>") {
		p.Message(l)
	}
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
			lastFetch := h.Ranks().LastRankFetch()
			if time.Since(lastFetch) < time.Second*5 {
				remaining := time.Second*5 - time.Since(lastFetch)
				p.SendTip(locale.Translate("rank.refetch.wait", fmt.Sprintf("%.1f", remaining.Seconds())))
				return
			}

			h.ranks.SetLastRankFetch(time.Now())
			p.SendTip(locale.Translate("rank.fetching"))
			go func() {
				h.Ranks().Load(p.XUID(), p.H())
			}()
		}
	}
}

// HandleChat ...
func (h *PlayerHandler) HandleChat(ctx *player.Context, message *string) {
	p := ctx.Val()
	ctx.Cancel()

	// TODO: Re-fetch moderation api, or cache the expiry date to ensure they are still muted.
	if h.inflictions.Muted() {
		p.Message(locale.Translate("mute.message"))
		return
	}

	// Only allow users with ranks other than Trainer to chat
	if h.Ranks().HighestRank() == rank.UnLinked {
		p.Message(locale.Translate("chat.discord.linked"))
		return
	}

	msg := h.Ranks().HighestRank().Chat(p.Name(), *message)
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

// Ranks ...
func (h *PlayerHandler) Ranks() *session.Ranks {
	return h.ranks
}

// Inflictions ...
func (h *PlayerHandler) Inflictions() *session.Inflictions {
	return h.inflictions
}
