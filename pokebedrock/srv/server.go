package srv

import (
	"log/slog"
	"strconv"

	"github.com/df-mc/atomic"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv/ping"
)

// Server ...
type Server struct {
	log *slog.Logger

	retries atomic.Int32
	conf    atomic.Value[Config]
	status  atomic.Value[Status]
}

// New ...
func New(log *slog.Logger, conf Config) *Server {
	srv := &Server{
		log: log,
	}
	srv.conf.Store(conf)
	return srv
}

// pingServer ...
func (s *Server) pingServer() {
	response, err := ping.Ping(s.Address())
	if err != nil {
		s.retries.Inc()
		if s.Retries() > 5 {
			s.assumeOffline()
			s.log.Warn("server assumed offline after multiple failures", "address", s.Address())
		} else {
			s.log.Debug("failed to ping server", "error", err, "retry", s.Retries())
		}
		return
	}

	s.retries.Store(0)

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
	return s.Config().Name
}

// Identifier ...
func (s *Server) Identifier() string {
	return s.Config().Identifier
}

// Icon ...
func (s *Server) Icon() string {
	return s.Config().Icon
}

// Address ...
func (s *Server) Address() string {
	return s.Config().Address
}

// Retries ...
func (s *Server) Retries() int32 {
	return s.retries.Load()
}

// Config ...
func (s *Server) Config() Config {
	return s.conf.Load()
}

// Status ...
func (s *Server) Status() Status {
	return s.status.Load()
}
