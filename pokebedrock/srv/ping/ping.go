// Package ping provides a ping service for the server.
package ping

import (
	"fmt"

	"github.com/sandertv/go-raknet"
)

const (
	// minPongFragments is the minimum number of fragments expected in a pong response
	minPongFragments = 7
)

// RakNetResponse represents the response data for a RakNet ping.
// It contains information about the server's game type, player count, and other details.
type RakNetResponse struct {
	GameType         string
	MessageOfTheDay  string
	ProtocolVersion  string
	MinecraftVersion string
	PlayerCount      string
	MaxPlayerCount   string
	ServerID         string
	ServerSoftware   string
	ServerGameMode   string
}

// Ping sends a ping to the specified server address and returns a RakNetResponse with the server's details.
// It returns an error if the ping or data parsing fails.
func Ping(address string) (RakNetResponse, error) {
	raw, err := raknet.Ping(address)
	if err != nil {
		return RakNetResponse{}, err
	}

	frag := splitPong(string(raw))
	if len(frag) < minPongFragments {
		return RakNetResponse{}, fmt.Errorf("invalid pong data")
	}

	return RakNetResponse{
		GameType:         frag[0],
		MessageOfTheDay:  frag[1],
		ProtocolVersion:  frag[2],
		MinecraftVersion: frag[3],
		PlayerCount:      frag[4],
		MaxPlayerCount:   frag[5],
		ServerID:         frag[6],
		ServerSoftware:   frag[7],
		ServerGameMode:   frag[8],
	}, nil
}

// splitPong splits the raw pong string into individual tokens.
// It also handles escape sequences and semicolons as token separators.
func splitPong(s string) []string {
	var runes []rune

	var tokens []string

	inEscape := false

	for _, r := range s {
		switch {
		case r == '\\':
			inEscape = true
		case r == ';':
			tokens = append(tokens, string(runes))
			runes = runes[:0]
		case inEscape:
			inEscape = false

			fallthrough
		default:
			runes = append(runes, r)
		}
	}

	return append(tokens, string(runes))
}
