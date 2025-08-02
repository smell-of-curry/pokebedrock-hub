package restart

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"
)

// globalService holds the global restart manager service instance.
var globalService *Service

// GlobalService returns the global service instance.
func GlobalService() *Service {
	return globalService
}

// Service manages server restart coordination and permissions.
type Service struct {
	log    *slog.Logger
	config Config
	closed bool
	mu     sync.RWMutex

	state ServerRestartState
}

// Config holds the configuration for the restart manager service.
type Config struct {
	MaxWaitTime     time.Duration // Maximum time a server will wait before force restart (default: 10 minutes)
	MaxFailures     int           // Maximum failures before force restart (default: 3)
	BackoffInterval time.Duration // Backoff interval between retries (default: 1 minute)
	RestartCooldown time.Duration // Minimum time between restarts for the same server (default: 5 minutes)
	QueueTimeout    time.Duration // Time after which queue entries expire (default: 15 minutes)
}

// DefaultConfig returns the default configuration for the restart manager.
func DefaultConfig() Config {
	return Config{
		MaxWaitTime:     10 * time.Minute,
		MaxFailures:     3,
		BackoffInterval: 1 * time.Minute,
		RestartCooldown: 5 * time.Minute,
		QueueTimeout:    15 * time.Minute,
	}
}

// NewService initializes a new global restart manager service instance.
func NewService(log *slog.Logger, config Config) {
	globalService = &Service{
		log:    log,
		config: config,
		closed: false,
		state: ServerRestartState{
			CurrentlyRestarting:  "",
			Queue:                make([]QueueEntry, 0),
			RestartHistory:       make(map[string]time.Time),
			UnauthorizedRestarts: make(map[string][]time.Time),
		},
	}

	// Start background cleanup process
	go globalService.cleanupExpiredEntries()

	log.Info("Restart Manager service initialized",
		"maxWaitTime", config.MaxWaitTime,
		"maxFailures", config.MaxFailures,
		"backoffInterval", config.BackoffInterval)
}

// RequestRestart handles a restart request from a downstream server.
func (s *Service) RequestRestart(req RestartRequest) RestartResponse {
	if s.closed {
		return RestartResponse{
			Status:       RestartStatusDeny,
			Message:      "Restart manager service is closed",
			ResponseTime: time.Now().Unix(),
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// If this server is already the currently restarting server, grant permission
	if s.state.CurrentlyRestarting == req.Host {
		s.log.Debug("Server restart request granted - already currently restarting", "host", req.Host)

		return RestartResponse{
			Status:       RestartStatusAllow,
			Message:      "Restart permission granted",
			ResponseTime: now.Unix(),
		}
	}

	// Check if server is in cooldown period
	if lastRestart, exists := s.state.RestartHistory[req.Host]; exists {
		if now.Sub(lastRestart) < s.config.RestartCooldown {
			remaining := s.config.RestartCooldown - now.Sub(lastRestart)
			s.log.Debug("Server restart request denied - cooldown period",
				"host", req.Host,
				"remaining", remaining)

			return RestartResponse{
				Status:       RestartStatusWait,
				Message:      fmt.Sprintf("Server in cooldown period, try again in %v", remaining.Round(time.Second)),
				RetryAfter:   now.Add(remaining).Unix(),
				ResponseTime: now.Unix(),
			}
		}
	}

	// If no server is currently restarting, allow this one
	if s.state.CurrentlyRestarting == "" {
		s.state.CurrentlyRestarting = req.Host
		s.state.RestartHistory[req.Host] = now

		s.log.Info("Server restart permission granted", "host", req.Host)

		return RestartResponse{
			Status:       RestartStatusAllow,
			Message:      "Restart permission granted",
			ResponseTime: now.Unix(),
		}
	}

	// Check if server is already in queue
	for i, entry := range s.state.Queue {
		if entry.Host == req.Host {
			// Update existing queue entry
			s.state.Queue[i].LastRetry = now
			s.state.Queue[i].RetryCount++

			position := i + 1
			s.log.Debug("Server restart request - already in queue",
				"host", req.Host,
				"position", position,
				"retryCount", s.state.Queue[i].RetryCount)

			return RestartResponse{
				Status:       RestartStatusWait,
				Message:      fmt.Sprintf("Server in restart queue (position %d)", position),
				QueuePos:     position,
				RetryAfter:   now.Add(s.config.BackoffInterval).Unix(),
				ResponseTime: now.Unix(),
			}
		}
	}

	// Add server to queue
	queueEntry := QueueEntry{
		Host:         req.Host,
		ServerName:   req.ServerName,
		RequestTime:  now,
		LastRetry:    now,
		RetryCount:   0,
		FailureCount: 0,
		Status:       RestartStatusWait,
	}

	s.state.Queue = append(s.state.Queue, queueEntry)
	position := len(s.state.Queue)

	s.log.Info("Server added to restart queue",
		"host", req.Host,
		"position", position)

	return RestartResponse{
		Status:       RestartStatusWait,
		Message:      fmt.Sprintf("Server added to restart queue (position %d)", position),
		QueuePos:     position,
		RetryAfter:   now.Add(s.config.BackoffInterval).Unix(),
		ResponseTime: now.Unix(),
	}
}

// NotifyRestartComplete notifies the service that a server has completed its restart.
func (s *Service) NotifyRestartComplete(notification RestartNotification) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.CurrentlyRestarting != notification.Host {
		s.log.Warn("Received restart completion from unexpected server",
			"expected", s.state.CurrentlyRestarting,
			"received", notification.Host)

		return fmt.Errorf("unexpected restart completion from server %s", notification.Host)
	}

	s.log.Info("Server restart completed",
		"host", notification.Host,
		"duration", time.Unix(notification.CompletedTime, 0).Sub(time.Unix(notification.RestartTime, 0)))

	// Clear currently restarting server
	s.state.CurrentlyRestarting = ""

	// Process next server in queue if any
	s.processQueue()

	return nil
}

// NotifyUnauthorizedRestart records that a server restarted without permission.
func (s *Service) NotifyUnauthorizedRestart(host string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.state.UnauthorizedRestarts[host] = append(s.state.UnauthorizedRestarts[host], now)
	s.state.RestartHistory[host] = now

	s.log.Warn("Server performed unauthorized restart",
		"host", host,
		"total_unauthorized", len(s.state.UnauthorizedRestarts[host]))

	// Remove from queue if present
	for i, entry := range s.state.Queue {
		if entry.Host == host {
			s.state.Queue = append(s.state.Queue[:i], s.state.Queue[i+1:]...)
			s.log.Debug("Removed server from queue due to unauthorized restart", "host", host)

			break
		}
	}

	// If this was the currently restarting server, clear it and process queue
	if s.state.CurrentlyRestarting == host {
		s.state.CurrentlyRestarting = ""
		s.processQueue()
	}
}

// processQueue processes the next server in the restart queue.
func (s *Service) processQueue() {
	if len(s.state.Queue) == 0 {
		return
	}

	// Get next server from queue
	nextServer := s.state.Queue[0]
	s.state.Queue = s.state.Queue[1:]

	// Set as currently restarting
	s.state.CurrentlyRestarting = nextServer.Host
	s.state.RestartHistory[nextServer.Host] = time.Now()

	s.log.Info("Processing next server from restart queue",
		"host", nextServer.Host,
		"waitTime", time.Since(nextServer.RequestTime))
}

// cleanupExpiredEntries removes expired entries from the queue periodically.
func (s *Service) cleanupExpiredEntries() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		if s.closed {
			return
		}

		<-ticker.C
		s.mu.Lock()
		now := time.Now()

		var validQueue []QueueEntry

		for _, entry := range s.state.Queue {
			// Remove entries that have been waiting too long
			if now.Sub(entry.RequestTime) < s.config.QueueTimeout {
				validQueue = append(validQueue, entry)
			} else {
				s.log.Debug("Removing expired queue entry",
					"host", entry.Host,
					"waitTime", now.Sub(entry.RequestTime))
			}
		}

		s.state.Queue = validQueue
		s.mu.Unlock()
	}
}

// GetState returns the current state of the restart manager (for debugging/monitoring).
func (s *Service) GetState() ServerRestartState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid race conditions
	state := ServerRestartState{
		CurrentlyRestarting:  s.state.CurrentlyRestarting,
		Queue:                make([]QueueEntry, len(s.state.Queue)),
		RestartHistory:       make(map[string]time.Time),
		UnauthorizedRestarts: make(map[string][]time.Time),
	}

	copy(state.Queue, s.state.Queue)
	maps.Copy(state.RestartHistory, s.state.RestartHistory)

	// Deep copy the UnauthorizedRestarts map of slices
	for host, times := range s.state.UnauthorizedRestarts {
		state.UnauthorizedRestarts[host] = make([]time.Time, len(times))
		copy(state.UnauthorizedRestarts[host], times)
	}

	return state
}

// GetStateJSON returns the current state as JSON (for API endpoints).
func (s *Service) GetStateJSON() ([]byte, error) {
	state := s.GetState()

	return json.MarshalIndent(state, "", "  ")
}

// Stop stops the restart manager service.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	s.log.Debug("Restart Manager service stopped")
}
