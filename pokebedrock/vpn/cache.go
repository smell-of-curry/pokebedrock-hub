package vpn

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"sync"
)

// Cache stores IP -> isProxy results and persists them to disk.
type Cache struct {
	mu   sync.RWMutex
	path string
	data map[string]bool
}

// NewCache creates a cache instance backed by the given file path. If the file
// exists, it will be loaded. If not, an empty cache is created.
func NewCache(path string) (*Cache, error) {
	c := &Cache{
		path: path,
		data: make(map[string]bool),
	}

	if path == "" {
		return c, nil
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Ensure directory exists for future writes
			_ = os.MkdirAll(filepath.Dir(path), 0o755)

			// Write empty file
			_ = writeJSONFile(path, c.data)

			return c, nil
		}
		return nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	if err := dec.Decode(&c.data); err != nil {
		// If decode fails, start empty but keep file for future saves
		c.data = make(map[string]bool)
	}

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

// Set stores the value and persists the cache to disk.
func (c *Cache) Set(ip string, isProxy bool) {
	if c == nil || c.path == "" {
		return
	}

	c.mu.Lock()
	if c.data == nil {
		c.data = make(map[string]bool)
	}
	c.data[ip] = isProxy
	// Take a snapshot to write outside the lock as much as possible
	snapshot := make(map[string]bool, len(c.data))
	maps.Copy(snapshot, c.data)
	c.mu.Unlock()

	// Ensure directory exists
	_ = os.MkdirAll(filepath.Dir(c.path), 0o755)

	// Best-effort write; ignore errors here to avoid blocking join path
	// but we still attempt to persist.
	_ = writeJSONFile(c.path, snapshot)
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
