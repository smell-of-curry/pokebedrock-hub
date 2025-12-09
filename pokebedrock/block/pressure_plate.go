package block

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/world"
)

// PressurePlate ...
type PressurePlate struct {
	// Type is the type of the pressure plate.
	Type PressurePlateType
	// Power specifies the redstone power level currently being produced by the pressure plate.
	Power int
}

// Model ...
func (PressurePlate) Model() world.BlockModel {
	return model.Carpet{}
}

// EncodeBlock ...
func (p PressurePlate) EncodeBlock() (string, map[string]any) {
	return "minecraft:" + p.Type.String() + "_pressure_plate", map[string]any{"redstone_signal": int32(p.Power)}
}

var pressurePlateHash = block.NextHash()

// Hash ...
func (p PressurePlate) Hash() (uint64, uint64) {
	return pressurePlateHash, uint64(p.Type.Uint8()) | uint64(p.Power)<<8
}

// pressurePlates ...
func pressurePlates() (plates []world.Block) {
	for _, t := range PressurePlateTypes() {
		for i := 0; i <= 15; i++ {
			plates = append(plates, PressurePlate{Type: t, Power: i})
		}
	}
	return
}
