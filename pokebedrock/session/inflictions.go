package session

import (
	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/moderation"
)

// Inflictions represents a player's inflictions like being muted or frozen.
type Inflictions struct {
	muted        atomic.Bool
	muteDuration atomic.Value[int64]
	frozen       atomic.Bool
}

// NewInflictions creates a new Inflictions object with default values.
func NewInflictions() *Inflictions {
	i := &Inflictions{}
	i.muted.Store(false)
	i.muteDuration.Store(0)
	i.frozen.Store(false)
	return i
}

// Load loads the player's inflictions from the moderation service.
func (i *Inflictions) Load(handle *world.EntityHandle) {
	handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
		p := e.(*player.Player)
		resp, err := moderation.GlobalService().InflictionOfPlayer(p)
		if err != nil {
			return
		}

		for _, infliction := range resp.CurrentInflictions {
			switch infliction.Type {
			case moderation.InflictionMuted:
				expiry := infliction.ExpiryDate
				if expiry != nil && *expiry != 0 {
					i.muteDuration.Store(*expiry)
				}
				i.muted.Store(true)
			case moderation.InflictionFrozen:
				i.frozen.Store(true)
			}
		}

		i.handleActiveInflictions(p)
	})
}

// handleActiveInflictions applies the effects of active inflictions on the player.
func (i *Inflictions) handleActiveInflictions(p *player.Player) {
	if i.Frozen() {
		p.SetImmobile()
	}
}

// SetMuted sets whether the player is muted or not.
func (i *Inflictions) SetMuted(muted bool) {
	i.muted.Store(muted)
}

// Muted returns whether the player is muted or not.
func (i *Inflictions) Muted() bool {
	return i.muted.Load()
}

// SetMuteDuration sets the mute duration for the player.
func (i *Inflictions) SetMuteDuration(duration int64) {
	i.muteDuration.Store(duration)
}

// MuteDuration returns the current mute duration.
func (i *Inflictions) MuteDuration() int64 {
	return i.muteDuration.Load()
}

// SetFrozen sets whether the player is frozen or not.
func (i *Inflictions) SetFrozen(frozen bool) {
	i.frozen.Store(frozen)
}

// Frozen returns whether the player is frozen or not.
func (i *Inflictions) Frozen() bool {
	return i.frozen.Load()
}
