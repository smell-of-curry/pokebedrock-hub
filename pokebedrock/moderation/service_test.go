package moderation

import "testing"

func TestHostFromAddress(t *testing.T) {
	tests := map[string]string{
		"127.0.0.1:19132": "127.0.0.1",
		"[::1]:19132":     "::1",
		"unknown":         "unknown",
	}
	for address, expected := range tests {
		if actual := hostFromAddress(address); actual != expected {
			t.Errorf("hostFromAddress(%q) = %q, want %q", address, actual, expected)
		}
	}
}
