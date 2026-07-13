package parkour

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func TestLeaderboardCloseFlushesLatestSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leaderboard.json")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	lb := newLeaderboard(log, path)

	if _, _, err := lb.update("course", "one", "One", "Member", 2*time.Second); err != nil {
		t.Fatal(err)
	}
	if _, _, err := lb.update("course", "two", "Two", "Member", time.Second); err != nil {
		t.Fatal(err)
	}
	lb.close()

	reloaded := newLeaderboard(log, path)
	defer reloaded.close()
	top := reloaded.top("course")
	if len(top) != 2 {
		t.Fatalf("expected two saved entries, got %d", len(top))
	}
	if top[0].XUID != "two" {
		t.Fatalf("expected latest fastest entry first, got %q", top[0].XUID)
	}
}
