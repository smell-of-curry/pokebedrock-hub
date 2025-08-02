package slapper

import (
	"sync"

	"github.com/df-mc/dragonfly/server/world"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/resources"
)

// Slappers is a map that stores all slappers by their identifier for easy lookup.
var slappers sync.Map

// SummonAll spawns slappers based on the provided configurations and stores them in the slappers map.
// Each slapper is spawned and initialized with the provided transaction and resource manager.
func SummonAll(cfgs []Config, tx *world.Tx, resManager *resources.Manager) {
	for _, c := range cfgs {
		s := NewSlapper(&c, resManager)
		s.Spawn(tx)
		slappers.Store(c.Identifier, s)
	}
}

// UpdateAll updates all slappers by calling their update method. It iterates over all slappers
// in the map and passes the transaction to each slapper's update method.
func UpdateAll(tx *world.Tx) {
	slappers.Range(func(_, value any) bool {
		if s, ok := value.(*Slapper); ok {
			s.update(tx)
		}

		return true
	})
}

// Register stores the given slapper in the slappers map using its identifier as the key.
func Register(s *Slapper) {
	slappers.Store(s.conf.Identifier, s)
}

// FromIdentifier returns a slapper by its unique identifier. If no slapper with the
// specified identifier exists, it returns nil.
func FromIdentifier(identifier string) *Slapper {
	if s, ok := slappers.Load(identifier); ok {
		return s.(*Slapper)
	}

	return nil
}

// All returns a map of all registered slappers, where the key is the identifier and the
// value is the corresponding slapper.
func All() map[string]*Slapper {
	result := make(map[string]*Slapper)

	slappers.Range(func(key, value any) bool {
		result[key.(string)] = value.(*Slapper)

		return true
	})

	return result
}
