package session

import "github.com/df-mc/atomic"

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
