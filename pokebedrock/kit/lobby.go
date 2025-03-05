package kit

import (
	"time"

	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/sandertv/gophertunnel/minecraft/text"
)

// Lobby ...
var Lobby lobby

// lobby ...
type lobby struct{}

// Items ...
func (lobby) Items(*player.Player) (items [36]item.Stack) {
	return [36]item.Stack{
		6: item.NewStack(item.Clock{}, 1).
			WithCustomName(text.Colourf("<green>Re-Fetch Synced Rank</green>")).
			WithValue("lobby", 2),
		7: item.NewStack(item.NetherStar{}, 1).
			WithCustomName(text.Colourf("<yellow>Back to Spawn</yellow>")).
			WithValue("lobby", 1),
		8: item.NewStack(item.Compass{}, 1).
			WithCustomName(text.Colourf("<purple>Server Navigator</purple>")).
			WithValue("lobby", 0),
	}
}

// ApplyFunc ...
func (lobby) ApplyFunc(p *player.Player) {
	_ = p.SetHeldSlot(8)
	p.ShowCoordinates()
	p.AddEffect(effect.New(effect.Speed, 5, time.Hour*24).WithoutParticles())

	p.SetGameMode(lobbyGameMode{})
	p.SetFlightSpeed(0.2)
	p.SetVerticalFlightSpeed(3)
}

// lobbyGameMode ...
type lobbyGameMode struct{}

func (lobbyGameMode) AllowsEditing() bool      { return false }
func (lobbyGameMode) AllowsTakingDamage() bool { return false }
func (lobbyGameMode) CreativeInventory() bool  { return false }
func (lobbyGameMode) HasCollision() bool       { return true }
func (lobbyGameMode) AllowsFlying() bool       { return true }
func (lobbyGameMode) AllowsInteraction() bool  { return true }
func (lobbyGameMode) Visible() bool            { return true }
