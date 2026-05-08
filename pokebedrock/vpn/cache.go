package vpn

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// defaultDirPerms is the default permission for created directories.
	defaultDirPerms = 0o755

	// flushInterval is the maximum delay between a Set call and the
	// resulting disk write. Coalesces bursts of inserts into one fsync.
	flushInterval = 5 * time.Second
)

// Cache stores IP -> isProxy results and persists them to disk.
//
// Writes are debounced: Set marks the cache dirty and signals a flusher
// goroutine, which writes the snapshot at most once per flushInterval.
// This avoids rewriting the entire JSON file on every player join.
type Cache struct {
	mu   sync.RWMutex
	path string
	data map[string]bool

	dirty    bool
	flush    chan struct{}
	stop     chan struct{}
	stopOnce sync.Once
	stopped  chan struct{}
}

// NewCache creates a cache instance backed by the given file path. If the
// file exists, it will be loaded.
func NewCache(path string) (*Cache, error) {
	c := &Cache{
		path:    path,
		data:    make(map[string]bool),
		flush:   make(chan struct{}, 1),
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}

	if path == "" {
		close(c.stopped)

		return c, nil
	}

	f, err := os.Open(path)
	switch {
	case err == nil:
		defer f.Close()

		dec := json.NewDecoder(f)
		if decErr := dec.Decode(&c.data); decErr != nil {
			// Keep the file but start with an empty in-memory map; the
			// next flush will rewrite a clean snapshot.
			c.data = make(map[string]bool)
		}
	case errors.Is(err, os.ErrNotExist):
		_ = os.MkdirAll(filepath.Dir(path), defaultDirPerms)
		_ = writeJSONFile(path, c.data)
	default:
		return nil, err
	}

	go c.flusher()

	return c, nil
}

// Get returns the cached value and whether it existed.
func (c *Cache) Get(ip string) (bool, bool) {
	if c == nil {
		return false, false
	}

	c.mu.RLock()
	v, ok := c.data[ip]
	c.mu.RUnlock()

	return v, ok
}

// Set stores the value and schedules a debounced disk write. Multiple Set
// calls within flushInterval coalesce into a single write.
func (c *Cache) Set(ip string, isProxy bool) {
	if c == nil || c.path == "" {
		return
	}

	c.mu.Lock()
	if c.data == nil {
		c.data = make(map[string]bool)
	}
	c.data[ip] = isProxy
	c.dirty = true
	c.mu.Unlock()

	// Non-blocking signal; the flusher will pick up the latest snapshot
	// regardless of how many times we signal.
	select {
	case c.flush <- struct{}{}:
	default:
	}
}

// Stop signals the flusher goroutine to drain any pending writes and
// exit. Safe to call from any goroutine and from multiple call sites.
func (c *Cache) Stop() {
	if c == nil {
		return
	}

	c.stopOnce.Do(func() {
		close(c.stop)
	})
	<-c.stopped
}

// flusher serialises all disk writes onto a single goroutine, coalescing
// bursts.
func (c *Cache) flusher() {
	defer close(c.stopped)

	timer := time.NewTimer(flushInterval)
	timer.Stop()

	for {
		select {
		case <-c.stop:
			c.writeIfDirty()

			return
		case <-c.flush:
			// Reset the timer to flushInterval from "now". When it
			// fires we flush.
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(flushInterval)
		case <-timer.C:
			c.writeIfDirty()
		}
	}
}

// writeIfDirty snapshots the cache and writes it to disk if there have
// been any unsaved changes.
func (c *Cache) writeIfDirty() {
	c.mu.Lock()
	if !c.dirty {
		c.mu.Unlock()

		return
	}
	snapshot := make(map[string]bool, len(c.data))
	maps.Copy(snapshot, c.data)
	c.dirty = false
	c.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(c.path), defaultDirPerms); err != nil {
		// Mark dirty again so we retry on the next signal.
		c.markDirty()

		return
	}
	if err := writeJSONFile(c.path, snapshot); err != nil {
		c.markDirty()
	}
}

func (c *Cache) markDirty() {
	c.mu.Lock()
	c.dirty = true
	c.mu.Unlock()
}

func writeJSONFile(path string, v any) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	err = enc.Encode(v)
	cerr := f.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)

		return err
	}

	return os.Rename(tmp, path)
}
