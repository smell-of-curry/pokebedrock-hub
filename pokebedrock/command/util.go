package command

import (
	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/session"
)

// rankAllower ...
type rankAllower struct {
	rank rank.Rank
}

// Allow ...
func (r rankAllower) Allow(s cmd.Source) bool {
	p, ok := s.(*player.Player)
	if !ok {
		return false
	}
	h, ok := p.Handler().(rankHandler)
	if !ok {
		return false
	}
	return h.Ranks().HighestRank() >= r.rank
}

// rankHandler ...
type rankHandler interface {
	Ranks() *session.Ranks
}
