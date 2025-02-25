package ping

import (
	"fmt"

	"github.com/sandertv/go-raknet"
)

// RakNetResponse ...
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

// Ping ...
func Ping(address string) (RakNetResponse, error) {
	raw, err := raknet.Ping(address)
	if err != nil {
		return RakNetResponse{}, err
	}
	frag := splitPong(string(raw))
	if len(frag) < 7 {
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

// splitPong ...
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
