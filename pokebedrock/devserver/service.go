package devserver

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

const (
	requestTimeout = 5 * time.Second
	defaultPoll    = 10 * time.Second
)

// Config holds DevServers settings from config.toml.
type Config struct {
	Enabled             bool
	URL                 string
	Token               string
	Host                string
	PollIntervalSeconds int
}

// Service polls the remote manager and syncs `dev-` servers into the registry.
type Service struct {
	cfg    Config
	client *http.Client
	log    *slog.Logger

	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

var (
	globalService *Service
	globalMu      sync.RWMutex
)

// GlobalService returns the singleton poller, or nil if not started.
func GlobalService() *Service {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalService
}

// NewService creates and starts the poller when Enabled and required fields are set.
func NewService(log *slog.Logger, cfg Config) *Service {
	if !cfg.Enabled {
		return nil
	}
	if cfg.URL == "" || cfg.Token == "" || cfg.Host == "" {
		log.Warn("dev servers enabled but URL/Token/Host incomplete; poller not started")
		return nil
	}

	interval := time.Duration(cfg.PollIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = defaultPoll
	}
	cfg.PollIntervalSeconds = int(interval / time.Second)

	s := &Service{
		cfg: cfg,
		client: &http.Client{
			Timeout: requestTimeout,
		},
		log:  log,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}

	globalMu.Lock()
	globalService = s
	globalMu.Unlock()

	go s.loop(interval)
	return s
}

// Stop ends the poll loop. Safe to call on nil.
func (s *Service) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stop)
	})
	select {
	case <-s.done:
	case <-time.After(3 * time.Second):
	}
}

func (s *Service) loop(interval time.Duration) {
	defer close(s.done)

	s.pollOnce()

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.pollOnce()
		}
	}
}

func (s *Service) pollOnce() {
	resp, err := s.fetch()
	if err != nil {
		s.log.Warn("dev servers poll failed; keeping current set", "error", err)
		return
	}

	desired, err := DesiredConfigs(resp.Servers, s.cfg.Host)
	if err != nil {
		s.log.Warn("dev servers response invalid; keeping current set", "error", err)
		return
	}

	current := currentDevSnapshots()
	register, unregister := Diff(current, desired)

	for _, id := range unregister {
		srv.Unregister(id)
		s.log.Info("unregistered dev server", "identifier", id)
	}
	for _, cfg := range register {
		srv.Register(srv.NewServer(s.log, cfg))
		s.log.Info("registered dev server", "identifier", cfg.Identifier, "name", cfg.Name, "address", cfg.Address)
	}
}

func (s *Service) fetch() (APIResponse, error) {
	base := strings.TrimRight(s.cfg.URL, "/")
	url := base + "/dev-servers"

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return APIResponse{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("authorization", s.cfg.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return APIResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return APIResponse{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return APIResponse{}, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return DecodeResponse(body)
}

func currentDevSnapshots() map[string]Snapshot {
	out := make(map[string]Snapshot)
	for _, s := range srv.All() {
		id := s.Identifier()
		if !IsDevIdentifier(id) {
			continue
		}
		cfg := s.Config()
		out[id] = Snapshot{Name: cfg.Name, Address: cfg.Address}
	}
	return out
}
