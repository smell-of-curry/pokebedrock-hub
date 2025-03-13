package command

import (
	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/form"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// Moderate represents a moderation command that can be executed by players with sufficient rank.
// It includes a target player to apply moderation actions to, as well as the rank requirement.
type Moderate struct {
	Target string `name:"target"`

	rankAllower
}

// NewModerate ...
func NewModerate(r rank.Rank) cmd.Command {
	return cmd.New("moderate", "", nil, Moderate{rankAllower: rankAllower{rank: r}})
}

// Run ...
func (m Moderate) Run(src cmd.Source, _ *cmd.Output, _ *world.Tx) {
	src.(*player.Player).SendForm(form.NewModerate(m.Target))
}
