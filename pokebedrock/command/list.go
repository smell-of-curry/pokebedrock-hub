package command

import (
	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// List represents a command that displays all online players.
type List struct {
	rankAllower
}

// NewList creates a new list command with the specified rank requirement.
func NewList(r rank.Rank) cmd.Command {
	return cmd.New("list", "Lists all online players", []string{"ls"}, List{rankAllower: rankAllower{rank: r}})
}

// Run executes the list command.
func (l List) Run(_ cmd.Source, o *cmd.Output, tx *world.Tx) {
	// Collect all players in the world
	players := make([]*player.Player, 0)

	for ent := range tx.Players() {
		p := ent.(*player.Player)
		players = append(players, p)
	}

	count := len(players)
	if count == 0 {
		o.Print("There are 0 players online")

		return
	}

	// Display the list of players
	if count == 1 {
		o.Print("There is 1 player online:")
	} else {
		o.Print("There are", "count", count, "players online:")
	}

	// Get player names, potentially with rank information if possible
	for _, p := range players {
		if h, ok := p.Handler().(rankHandler); ok {
			// If the player has a rank handler, display their name with rank color/format
			r := h.Ranks().HighestRank()
			o.Print(text.Colourf(" - %s", r.FormatName(p.Name())))
		} else {
			// Fallback to just showing the player name
			o.Print(" -", "player", p.Name())
		}
	}
}
