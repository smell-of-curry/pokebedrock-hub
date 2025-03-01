package queue

import (
	"time"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

// Entry ...
type Entry struct {
	joinTime time.Time
	index    int

	handle *world.EntityHandle
	rank   rank.Rank
	srv    *srv.Server
}
