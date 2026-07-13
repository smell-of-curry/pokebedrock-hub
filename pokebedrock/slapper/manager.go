package slapper

import (
	"errors"
	"sync"

	"github.com/df-mc/dragonfly/server/world"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/resources"
)

// Slappers is a map that stores all slappers by their identifier for easy lookup.
var slappers sync.Map

// LoadAll reads and parses slapper assets off the world owner.
func LoadAll(cfgs []Config, resManager *resources.Manager) ([]*Slapper, error) {
	loaded := make([]*Slapper, 0, len(cfgs))
	var loadErrors []error
	for _, c := range cfgs {
		s, err := NewSlapper(&c, resManager)
		if err != nil {
			loadErrors = append(loadErrors, err)
			continue
		}
		loaded = append(loaded, s)
	}
	return loaded, errors.Join(loadErrors...)
}

// SummonAll spawns preloaded slappers and stores them by identifier.
func SummonAll(loaded []*Slapper, tx *world.Tx) {
	for _, s := range loaded {
		if s == nil {
			continue
		}
		s.Spawn(tx)
		Register(s)
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
