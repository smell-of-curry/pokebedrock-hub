// Package authentication provides a thread-safe storage for player identities.
package authentication

import (
	"sync"
	"time"
)

const (
	// cleanupInterval is how often expired identities are cleaned up
	cleanupInterval = 5 * time.Minute

	// DefaultAuthDuration is the default duration for authentication tokens
	DefaultAuthDuration = 5 * time.Minute
)

// globalFactory is the singleton instance of the Factory used throughout the application.
var globalFactory *Factory

// GlobalFactory returns the global singleton Factory instance for managing player identities.
func GlobalFactory() *Factory {
	return globalFactory
}

func init() {
	globalFactory = &Factory{
		data: make(map[string]PlayerIdentity),
	}
	go globalFactory.startCleanup(cleanupInterval)
}

// Factory provides a thread-safe storage for player identities.
// It maps player xuid to their identity information and manages
// the lifecycle of these identities.
type Factory struct {
	data map[string]PlayerIdentity
	mu   sync.RWMutex
}

// startCleanup begins a periodic cleanup routine that removes expired identities.
// The interval parameter determines how often cleanup occurs.
func (f *Factory) startCleanup(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for range t.C {
		f.mu.Lock()
		now := time.Now()

		for xuid, identity := range f.data {
			if now.After(identity.Expiration) {
				delete(f.data, xuid)
			}
		}
		f.mu.Unlock()
	}
}

// Set ...
func (f *Factory) Set(name, xuid string, duration time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[xuid] = PlayerIdentity{
		DisplayName: name,
		XUID:        xuid,
		Expiration:  time.Now().Add(duration),
	}
}

// Of ...
func (f *Factory) Of(xuid string) (PlayerIdentity, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	req, exists := f.data[xuid]

	return req, exists
}

// Remove ...
func (f *Factory) Remove(xuid string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, xuid)
}
