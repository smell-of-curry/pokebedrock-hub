package srv

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/df-mc/atomic"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv/ping"
)

const (
	// maxRetries is the maximum number of ping failures before assuming server is offline
	maxRetries = 5
)

// Server represents a Minecraft server with configuration, retries, and online status.
// It provides methods for interacting with server properties and monitoring its status.
type Server struct {
	log *slog.Logger

	retries atomic.Int32
	conf    atomic.Value[Config]
	status  atomic.Value[Status]
}

// NewServer creates and returns a new Server instance with the provided logger and configuration.
func NewServer(log *slog.Logger, conf Config) *Server {
	srv := &Server{
		log: log,
	}
	srv.conf.Store(conf)

	return srv
}

// pingServer pings the server to check if it's online and updates the status accordingly.
// If the ping fails repeatedly, the server is assumed offline.
func (s *Server) pingServer() {
	response, err := ping.Ping(s.Address())
	if err != nil {
		s.retries.Inc()

		if s.Retries() > maxRetries {
			s.assumeOffline()
			s.log.Debug("server assumed offline after multiple failures", "name", s.Name(), "address", s.Address())
			s.retries.Store(0)
		}

		return
	}

	st := Status{
		Online: true,
		PlayerCount: func() int {
			count, _ := strconv.Atoi(response.PlayerCount)

			return count
		}(),
		MaxPlayerCount: func() int {
			count, _ := strconv.Atoi(response.MaxPlayerCount)

			return count
		}(),
	}
	s.status.Store(st)
}

// assumeOffline marks the server as offline in its status.
func (s *Server) assumeOffline() {
	st := Status{
		Online: false,
	}
	s.status.Store(st)
}

// Name returns the server's name from its configuration.
func (s *Server) Name() string {
	return s.Config().Name
}

// Identifier returns the server's unique identifier from its configuration.
func (s *Server) Identifier() string {
	return s.Config().Identifier
}

// Icon returns the server's icon (e.g., URL or base64 data) from its configuration.
func (s *Server) Icon() string {
	return fmt.Sprintf("textures/ui/server_logos/%s", s.Config().Identifier)
}

// Address returns the server's address from its configuration.
func (s *Server) Address() string {
	return s.Config().Address
}

// Retries returns the current number of retries for the server ping.
func (s *Server) Retries() int32 {
	return s.retries.Load()
}

// Config returns the server's configuration.
func (s *Server) Config() Config {
	return s.conf.Load()
}

// Status returns the server's current status.
func (s *Server) Status() Status {
	return s.status.Load()
}
