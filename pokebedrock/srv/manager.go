package srv

import (
	"sync"
)

// Servers ...
var servers sync.Map

// Register ...
func Register(serv *Server) {
	servers.Store(serv.Identifier(), serv)
}

// UpdateAll ...
func UpdateAll() {
	servers.Range(func(_, value any) bool {
		if srv, ok := value.(*Server); ok {
			go srv.pingServer()
		}
		return true
	})
}

// FromIdentifier ...
func FromIdentifier(identifier string) *Server {
	if srv, ok := servers.Load(identifier); ok {
		return srv.(*Server)
	}
	return nil
}

// FromName ...
func FromName(name string) *Server {
	for _, srv := range All() {
		if srv.Name() == name {
			return srv
		}
	}
	return nil
}

// All ...
func All() map[string]*Server {
	result := make(map[string]*Server)
	servers.Range(func(key, value any) bool {
		result[key.(string)] = value.(*Server)
		return true
	})
	return result
}
