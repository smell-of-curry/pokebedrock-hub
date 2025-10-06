// Package slapper provides a configuration for a slapper.
package slapper

// Config represents the configuration for a slapper, including its name, identifier,
// scale, position, and rotation.
type Config struct {
	Name       string
	Identifier string

	Scale    float64
	Yaw      float64
	Pitch    float64
	Position struct {
		X float64
		Y float64
		Z float64
	}
}
