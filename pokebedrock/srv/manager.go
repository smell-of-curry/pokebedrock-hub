package srv

import "sync"

// Servers ...
var servers sync.Map

// Register ...
func Register(serv *Server) {
	servers.Store(serv.conf.Identifier, serv)
}

// UpdateAll ...
func UpdateAll() {
	servers.Range(func(_, value any) bool {
		if s, ok := value.(*Server); ok {
			go s.pingServer()
		}
		return true
	})
}

// FromIdentifier ...
func FromIdentifier(identifier string) *Server {
	if serv, ok := servers.Load(identifier); ok {
		return serv.(*Server)
	}
	return nil
}

// FromName ...
func FromName(name string) *Server {
	for _, s := range All() {
		if s.Name() == name {
			return s
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
