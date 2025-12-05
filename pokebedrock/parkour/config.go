package parkour

import "github.com/go-gl/mathgl/mgl64"

// Config ...
type Config struct {
	LeaderboardPath  string
	CountdownSeconds int
	CompletionRadius float64
	Courses          []CourseConfig
}

// CourseConfig ...
type CourseConfig struct {
	Name, Identifier string
	NPC              NPCConfig
	Leaderboard      PositionConfig
	Start            PositionConfig
	End              PositionConfig
}

// NPCConfig ...
type NPCConfig struct {
	Scale    float64
	Yaw      float64
	Pitch    float64
	Position PositionConfig
}

// PositionConfig ...
type PositionConfig struct {
	X float64
	Y float64
	Z float64
}

// vec3 ...
func (x PositionConfig) vec3() mgl64.Vec3 {
	return mgl64.Vec3{x.X, x.Y, x.Z}
}
