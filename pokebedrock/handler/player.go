package handler

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/chat"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/form"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/hider"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/internal"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/kit"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/parkour"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/session"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/slapper"
)

// PlayerHandler ...
type PlayerHandler struct {
	ranks       *session.Ranks
	inflictions *session.Inflictions
	movement    *session.Movement

	player.NopHandler
}

// NewPlayerHandler creates a new player handler with initialized ranks and inflictions.
func NewPlayerHandler(p *player.Player) *PlayerHandler {
	h := &PlayerHandler{
		ranks:       session.NewRanks(),
		inflictions: session.NewInflictions(),
		movement:    session.NewMovement(),
	}

	// Add a small random delay to infliction loading to avoid all players
	// loading their inflictions at the exact same time
	go func() {
		// Random delay between 100ms and 2000ms to space out requests
		delay := time.Duration(internal.MinRandomDelayMs+rand.Intn(internal.MaxRandomDelayRangeMs)) * time.Millisecond
		time.Sleep(delay)
		h.inflictions.Load(p.H())
	}()

	// Add another random delay to rank loading
	go func() {
		// Random delay between 500ms and 3000ms
		delay := time.Duration(internal.MinRandomDelayLongMs+rand.Intn(internal.MaxRandomDelayLongRangeMs)) * time.Millisecond
		time.Sleep(delay)
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

	for _, s := range slapper.All() {
		s.SendAnimation(p)
	}

	hider.Global().HandleJoin(p)
}

// HandleItemUse ...
func (h *PlayerHandler) HandleItemUse(ctx *player.Context) {
	p := ctx.Val()
	it, _ := p.HeldItems()

	if id, exists := it.Value("lobby"); exists {
		action, _ := id.(string)
		switch action {
		case "navigator":
			p.SendForm(form.NewServerNavigator())
		case "spawn":
			w := p.Tx().World()
			p.Teleport(w.Spawn().Vec3Middle())
		case "sync-rank":
			lastFetch := h.Ranks().LastRankFetch()
			if time.Since(lastFetch) < time.Second*5 {
				remaining := time.Second*5 - time.Since(lastFetch)
				p.SendJukeboxPopup(locale.Translate("rank.refetch.wait", fmt.Sprintf("%.1f", remaining.Seconds())))
				return
			}

			h.ranks.SetLastRankFetch(time.Now())
			p.SendJukeboxPopup(locale.Translate("rank.fetching"))

			go func() {
				h.Ranks().Load(p.XUID(), p.H())
			}()
		case "toggle-visibility":
			hider.Global().Toggle(p)
		}
		return
	}

	if id, exists := it.Value("parkour"); exists {
		action, _ := id.(string)
		parkour.Global().HandleItemUse(p, action)
	}
}

// HandleChat ...
func (h *PlayerHandler) HandleChat(ctx *player.Context, message *string) {
	p := ctx.Val()
	ctx.Cancel()

	if h.inflictions.Muted() {
		dur := h.inflictions.MuteDuration()
		if dur == 0 || dur >= time.Now().UnixMilli() {
			p.Message(locale.Translate("mute.message"))

			return
		}
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

// HandleMove ...
func (h *PlayerHandler) HandleMove(ctx *player.Context, pos mgl64.Vec3, rot cube.Rotation) {
	p := ctx.Val()

	delta := pos.Sub(p.Position())
	if mgl64.FloatEqual(delta.X(), 0) && mgl64.FloatEqual(delta.Z(), 0) {
		// No horizontal movement, just return.
		return
	}

	parkour.Global().HandleMove(p, pos)

	movement := h.movement
	movement.SetLastMoveTime(time.Now())
	movement.SetLastPosition(pos)
	movement.SetLastRotation(rot)
}

// HandleQuit ...
func (h *PlayerHandler) HandleQuit(p *player.Player) {
	parkour.Global().HandleQuit(p)
	hider.Global().HandleQuit(p)
}

// Ranks ...
func (h *PlayerHandler) Ranks() *session.Ranks {
	return h.ranks
}

// Inflictions ...
func (h *PlayerHandler) Inflictions() *session.Inflictions {
	return h.inflictions
}

// Movement ...
func (h *PlayerHandler) Movement() *session.Movement {
	return h.movement
}
