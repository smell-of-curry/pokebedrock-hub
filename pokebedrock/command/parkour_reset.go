package command

import (
	"strings"

	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/parkour"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// ParkourReset ...
type ParkourReset struct {
	Course string               `name:"course"`
	XUID   cmd.Optional[string] `name:"xuid"`

	rankAllower
}

// NewParkourReset ...
func NewParkourReset(r rank.Rank) cmd.Command {
	return cmd.New("parkour_reset", "", nil, ParkourReset{rankAllower: rankAllower{rank: r}})
}

// Run ...
func (p ParkourReset) Run(_ cmd.Source, o *cmd.Output, _ *world.Tx) {
	course := strings.TrimSpace(p.Course)

	target, exists := p.XUID.Load()
	parkour.Global().Reset(course, target)

	if exists {
		o.Print(text.Colourf("<green>You've reset the '%s' course entries for %s.</green>", course, target))
		return
	}
	o.Print(text.Colourf("<green>You've reset all '%s' course entries.</green>", course))
}
