package authentication

import (
	"sync"
	"time"
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
	globalFactory.startCleanup(time.Minute * 5)
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
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			f.mu.Lock()
			now := time.Now()
			for name, identity := range f.data {
				if now.After(identity.Expiration) {
					delete(f.data, name)
				}
			}
			f.mu.Unlock()
		}
	}()
}

// Set ...
func (f *Factory) Set(name string, xuid string, duration time.Duration) {
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
