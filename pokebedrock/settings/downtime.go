// Package settings holds hub-wide runtime flags loaded from config.toml.
package settings

import "sync/atomic"

var downtimeLock atomic.Bool

// SetDowntimeLock sets whether downstream servers are locked to Sr. Moderator and above.
func SetDowntimeLock(enabled bool) {
	downtimeLock.Store(enabled)
}

// DowntimeLock reports whether downstream server access is restricted to Sr. Moderator and above.
func DowntimeLock() bool {
	return downtimeLock.Load()
}
