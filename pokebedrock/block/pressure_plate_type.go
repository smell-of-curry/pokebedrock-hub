package block

import "github.com/df-mc/dragonfly/server/block"

// PressurePlateType represents a type of pressure plate.
type PressurePlateType struct {
	pressurePlate

	// Wood is the type of wood of the pressure plate.
	wood block.WoodType
}

// pressurePlate ...
type pressurePlate uint8

// WoodPressurePlate returns the wood pressure plate type.
func WoodPressurePlate(w block.WoodType) PressurePlateType {
	return PressurePlateType{0, w}
}

// StonePressurePlate returns the stone pressure plate type.
func StonePressurePlate() PressurePlateType {
	return PressurePlateType{pressurePlate: 1}
}

// PolishedBlackstonePressurePlate returns the polished blackstone pressure plate type.
func PolishedBlackstonePressurePlate() PressurePlateType {
	return PressurePlateType{pressurePlate: 2}
}

// HeavyWeightedPressurePlate returns the heavy weighted pressure plate type.
func HeavyWeightedPressurePlate() PressurePlateType {
	return PressurePlateType{pressurePlate: 3}
}

// LightWeightedPressurePlate returns the light weighted pressure plate type.
func LightWeightedPressurePlate() PressurePlateType {
	return PressurePlateType{pressurePlate: 4}
}

// Uint8 ...
func (p PressurePlateType) Uint8() uint8 {
	return p.wood.Uint8() | uint8(p.pressurePlate)<<4
}

// Name ...
func (p PressurePlateType) Name() string {
	switch p.pressurePlate {
	case 0:
		return p.wood.Name() + " Pressure Plate"
	case 1:
		return "Stone Pressure Plate"
	case 2:
		return "Polished Blackstone Pressure Plate"
	case 3:
		return "Heavy Weighted Pressure Plate"
	case 4:
		return "Light Weighted Pressure Plate"
	}
	panic("unknown pressure plate type")
}

// String ...
func (p PressurePlateType) String() string {
	switch p.pressurePlate {
	case 0:
		if p.wood == block.OakWood() {
			return "wooden"
		}
		return p.wood.String()
	case 1:
		return "stone"
	case 2:
		return "polished_blackstone"
	case 3:
		return "heavy_weighted"
	case 4:
		return "light_weighted"
	}
	panic("unknown pressure plate type")
}

// PressurePlateTypes ...
func PressurePlateTypes() []PressurePlateType {
	types := []PressurePlateType{StonePressurePlate(), PolishedBlackstonePressurePlate(), HeavyWeightedPressurePlate(), LightWeightedPressurePlate()}
	for _, w := range block.WoodTypes() {
		types = append(types, WoodPressurePlate(w))
	}
	return types
}
