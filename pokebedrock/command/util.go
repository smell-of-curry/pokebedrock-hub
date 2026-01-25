package command

import (
	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/player"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/session"
)

// rankAllower is a structure that holds the rank requirement to allow or disallow certain actions or commands.
// It ensures that only players with the required rank or higher are able to execute the associated actions.
type rankAllower struct {
	rank rank.Rank
}

// Allow checks whether the source (player) has the required rank to perform the action.
// It returns true if the player has a rank equal to or higher than the required rank.
func (r rankAllower) Allow(s cmd.Source) bool {
	p, ok := s.(*player.Player)
	if !ok {
		return false
	}

	h, ok := p.Handler().(rankHandler)
	if !ok {
		return false
	}

	ranks := h.Ranks()
	if ranks == nil {
		return false
	}

	return ranks.HighestRank() >= r.rank
}

// rankHandler ...
type rankHandler interface {
	Ranks() *session.Ranks
}
