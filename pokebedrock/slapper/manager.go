package slapper

import (
	"log/slog"
	"sync"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/resources"
)

// Slappers ...
var slappers sync.Map

// SummonAll ...
func SummonAll(log *slog.Logger, cfgs []Config, tx *world.Tx, resManager *resources.Manager) {
	for _, c := range cfgs {
		s := New(log, &c, resManager)
		s.Spawn(tx)
		slappers.Store(c.Identifier, s)
	}
}

// UpdateAll ...
func UpdateAll(tx *world.Tx) {
	slappers.Range(func(_, value any) bool {
		if s, ok := value.(*Slapper); ok {
			s.update(tx)
		}
		return true
	})
}

// Register ...
func Register(s *Slapper) {
	slappers.Store(s.conf.Identifier, s)
}

// FromIdentifier ...
func FromIdentifier(identifier string) *Slapper {
	if s, ok := slappers.Load(identifier); ok {
		return s.(*Slapper)
	}
	return nil
}

// All ...
func All() map[string]*Slapper {
	result := make(map[string]*Slapper)
	slappers.Range(func(key, value any) bool {
		result[key.(string)] = value.(*Slapper)
		return true
	})
	return result
}
