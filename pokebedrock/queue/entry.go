package queue

import (
	"time"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// Entry ...
type Entry struct {
	joinTime time.Time
	index    int

	handle          *world.EntityHandle
	rank            rank.Rank
	transferAddress string
}
