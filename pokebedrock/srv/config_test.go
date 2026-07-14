package srv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigRejectsInvalidAddress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.json")
	if err := os.WriteFile(path, []byte(`{"address":"invalid"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseConfig(path); err == nil {
		t.Fatal("expected invalid server address error")
	}
}
