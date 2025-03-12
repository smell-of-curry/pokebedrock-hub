package session

import (
	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/moderation"
)

// Inflictions ...
type Inflictions struct {
	muted  atomic.Bool
	frozen atomic.Bool
}

// NewInflictions ...
func NewInflictions() *Inflictions {
	i := &Inflictions{}
	i.muted.Store(false)
	i.frozen.Store(false)
	return i
}

// Load ...
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
				i.muted.Store(true)
			case moderation.InflictionFrozen:
				i.frozen.Store(true)
			}
		}

		i.handleActiveInflictions(p)
	})
}

// handleActiveInflictions ...
func (i *Inflictions) handleActiveInflictions(p *player.Player) {
	if i.Frozen() {
		p.SetImmobile()
	}
}

// SetMuted ...
func (i *Inflictions) SetMuted(muted bool) {
	i.muted.Store(muted)
}

// Muted ...
func (i *Inflictions) Muted() bool {
	return i.muted.Load()
}

// SetFrozen ...
func (i *Inflictions) SetFrozen(frozen bool) {
	i.frozen.Store(frozen)
}

// Frozen ...
func (i *Inflictions) Frozen() bool {
	return i.frozen.Load()
}
