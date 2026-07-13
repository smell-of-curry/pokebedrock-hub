package parkour

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLeaderboardCloseFlushesLatestSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leaderboard.json")
	lb := newLeaderboard(path)

	if _, _, err := lb.update("course", "one", "One", "Member", 2*time.Second); err != nil {
		t.Fatal(err)
	}
	if _, _, err := lb.update("course", "two", "Two", "Member", time.Second); err != nil {
		t.Fatal(err)
	}
	lb.close()

	reloaded := newLeaderboard(path)
	defer reloaded.close()
	top := reloaded.top("course")
	if len(top) != 2 {
		t.Fatalf("expected two saved entries, got %d", len(top))
	}
	if top[0].XUID != "two" {
		t.Fatalf("expected latest fastest entry first, got %q", top[0].XUID)
	}
}
