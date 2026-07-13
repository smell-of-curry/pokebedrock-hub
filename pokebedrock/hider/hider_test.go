package hider

import (
	"io"
	"log/slog"
	"testing"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

func TestExemptedPlayersSkipsServerWithoutSlapper(t *testing.T) {
	srv.Register(srv.NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		srv.Config{Identifier: "missing-slapper"},
	))

	if handles := NewManager().exemptedPlayers(); len(handles) != 0 {
		t.Fatalf("expected no exempted handles, got %d", len(handles))
	}
}
