package identity

import (
	"sync"
	"time"
)

// globalFactory ...
var globalFactory *Factory

// GlobalFactory ...
func GlobalFactory() *Factory {
	return globalFactory
}

func init() {
	globalFactory = &Factory{
		data: make(map[string]PlayerIdentity),
	}
}

// Factory ...
type Factory struct {
	data map[string]PlayerIdentity
	mu   sync.RWMutex
}

// Set ...
func (f *Factory) Set(name string, xuid string, duration time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.data[name] = PlayerIdentity{
		ThirdPartyName: name,
		XUID:           xuid,
		Expiration:     time.Now().Add(duration),
	}
}

// Of ...
func (f *Factory) Of(name string) (PlayerIdentity, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	req, exists := f.data[name]
	return req, exists
}

// Remove ...
func (f *Factory) Remove(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, name)
}
