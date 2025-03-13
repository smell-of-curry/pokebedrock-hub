package srv

import (
	"sort"
	"sync"

	"github.com/sandertv/gophertunnel/minecraft/text"
)

// servers holds all the registered server instances in a sync.Map for thread-safe access.
var servers sync.Map

// Register adds a new server to the list of registered servers using its identifier as the key.
func Register(srv *Server) {
	servers.Store(srv.Identifier(), srv)
}

// UpdateAll iterates over all registered servers and invokes the pingServer method concurrently.
func UpdateAll() {
	servers.Range(func(_, value any) bool {
		if srv, ok := value.(*Server); ok {
			go srv.pingServer()
		}
		return true
	})
}

// FromIdentifier retrieves a server by its identifier.
func FromIdentifier(identifier string) *Server {
	if srv, ok := servers.Load(identifier); ok {
		return srv.(*Server)
	}
	return nil
}

// FromName searches all registered servers by their name and returns the first match.
func FromName(name string) *Server {
	for _, srv := range All() {
		if text.Clean(srv.Name()) == name {
			return srv
		}
	}
	return nil
}

// All returns a slice of all registered servers sorted by their identifier.
func All() []*Server {
	var result []*Server
	servers.Range(func(key, value any) bool {
		result = append(result, value.(*Server))
		return true
	})

	// Sort the servers by identifier for consistent ordering.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Identifier() < result[j].Identifier()
	})
	return result
}
