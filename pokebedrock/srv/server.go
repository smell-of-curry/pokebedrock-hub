package srv

import (
	"log/slog"
	"strconv"

	"github.com/df-mc/atomic"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv/ping"
)

// Server ...
type Server struct {
	log  *slog.Logger
	conf *Config

	status atomic.Value[Status]
}

// New ...
func New(log *slog.Logger, conf *Config) *Server {
	return &Server{
		log:  log,
		conf: conf,
	}
}

// pingServer ...
func (s *Server) pingServer() {
	response, err := ping.Ping(s.conf.Address)
	if err != nil {
		s.assumeOffline()
		s.log.Debug("failed to ping server", "error", err)
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

// assumeOffline ...
func (s *Server) assumeOffline() {
	st := Status{
		Online: false,
	}
	s.status.Store(st)
}

// Name ...
func (s *Server) Name() string {
	return s.conf.Name
}

// Address ...
func (s *Server) Address() string {
	return s.conf.Address
}

// Status ...
func (s *Server) Status() Status {
	return s.status.Load()
}
