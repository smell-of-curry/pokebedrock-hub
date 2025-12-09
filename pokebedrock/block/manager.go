package block

import "github.com/df-mc/dragonfly/server/world"

func init() {
	for _, plate := range pressurePlates() {
		world.RegisterBlock(plate)
	}
}
