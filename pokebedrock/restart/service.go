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

	state ServerState
}

// Config holds the configuration for the restart manager service.
type Config struct {
	MaxWaitTime     time.Duration // Maximum time a server will wait before force restart (default: 10 minutes)
	BackoffInterval time.Duration // Backoff interval between retries (default: 1 minute)
	RestartCooldown time.Duration // Minimum time between restarts for the same server (default: 5 minutes)
	QueueTimeout    time.Duration // Time after which queue entries expire (default: 15 minutes)
	MaxRestartTime  time.Duration // Maximum time a server is allowed to be in restarting state (default: 20 minutes)
}

// DefaultConfig returns the default configuration for the restart manager.
func DefaultConfig() Config {
	return Config{
		MaxWaitTime:     10 * time.Minute,
		BackoffInterval: 1 * time.Minute,
		RestartCooldown: 5 * time.Minute,
		QueueTimeout:    15 * time.Minute,
		MaxRestartTime:  20 * time.Minute,
	}
}

// NewService initializes a new global restart manager service instance.
func NewService(log *slog.Logger, config Config) {
	globalService = &Service{
		log:    log,
		config: config,
		closed: false,
		state: ServerState{
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
		"backoffInterval", config.BackoffInterval,
		"maxRestartTime", config.MaxRestartTime)
}

// RequestRestart handles a restart request from a downstream server.
func (s *Service) RequestRestart(req Request) Response {
	if s.closed {
		return Response{
			Status:  StatusDeny,
			Message: "Restart manager service is closed",
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Check if server is already currently restarting
	if response, handled := s.checkCurrentlyRestarting(req.ServerName); handled {
		return response
	}

	// Check cooldown period
	if response, handled := s.checkCooldownPeriod(req.ServerName, now); handled {
		return response
	}

	// If no server is currently restarting, allow this one
	if s.state.CurrentlyRestarting == "" {
		return s.grantRestartPermission(req.ServerName, now)
	}

	// Handle queue operations
	return s.handleQueueOperations(req.ServerName, now)
}

// checkCurrentlyRestarting checks if the server is already currently restarting
func (s *Service) checkCurrentlyRestarting(serverName string) (Response, bool) {
	if s.state.CurrentlyRestarting == serverName {
		s.log.Debug("Server restart request granted - already currently restarting", "name", serverName)
		
		return Response{
			Status:  StatusAllow,
			Message: "Restart permission granted",
		}, true
	}
	return Response{}, false
}

// checkCooldownPeriod checks if the server is in cooldown period
func (s *Service) checkCooldownPeriod(serverName string, now time.Time) (Response, bool) {
	lastRestart, exists := s.state.RestartHistory[serverName]
	if !exists {
		return Response{}, false
	}

	if now.Sub(lastRestart) < s.config.RestartCooldown {
		cooldownEnd := lastRestart.Add(s.config.RestartCooldown)
		remaining := cooldownEnd.Sub(now)
		s.log.Debug("Server restart request denied - cooldown period",
			"name", serverName,
			"remaining", remaining)

		return Response{
			Status:     StatusWait,
			Message:    fmt.Sprintf("Server in cooldown period, try again in %v", remaining.Round(time.Second)),
			RetryAfter: cooldownEnd.UTC().UnixMilli(),
		}, true
	}
	return Response{}, false
}

// grantRestartPermission grants restart permission to the server
func (s *Service) grantRestartPermission(serverName string, now time.Time) Response {
	s.state.CurrentlyRestarting = serverName
	s.state.RestartHistory[serverName] = now

	s.log.Info("Server restart permission granted", "name", serverName)

	return Response{
		Status:  StatusAllow,
		Message: "Restart permission granted",
	}
}

// handleQueueOperations handles queue-related operations for restart requests
func (s *Service) handleQueueOperations(serverName string, now time.Time) Response {
	// Check if server is already in queue
	for i, entry := range s.state.Queue {
		if entry.ServerName == serverName {
			return s.updateExistingQueueEntry(i, now)
		}
	}

	// Add server to queue
	return s.addServerToQueue(serverName, now)
}

// updateExistingQueueEntry updates an existing queue entry
func (s *Service) updateExistingQueueEntry(index int, now time.Time) Response {
	s.state.Queue[index].LastRetry = now
	s.state.Queue[index].RetryCount++

	position := index + 1
	s.log.Debug("Server restart request - already in queue",
		"name", s.state.Queue[index].ServerName,
		"position", position,
		"retryCount", s.state.Queue[index].RetryCount)

	return Response{
		Status:     StatusWait,
		Message:    "Server in restart queue",
		QueuePos:   position,
		RetryAfter: now.Add(s.config.BackoffInterval * time.Duration(position)).UTC().UnixMilli(),
	}
}

// addServerToQueue adds a new server to the restart queue
func (s *Service) addServerToQueue(serverName string, now time.Time) Response {
	queueEntry := QueueEntry{
		ServerName:   serverName,
		RequestTime:  now,
		LastRetry:    now,
		RetryCount:   0,
		FailureCount: 0,
		Status:       StatusWait,
	}

	s.state.Queue = append(s.state.Queue, queueEntry)
	position := len(s.state.Queue)

	s.log.Info("Server added to restart queue",
		"name", serverName,
		"position", position)

	return Response{
		Status:     StatusWait,
		Message:    fmt.Sprintf("Server added to restart queue (position %d)", position),
		QueuePos:   position,
		RetryAfter: now.Add(s.config.BackoffInterval * time.Duration(position)).UTC().UnixMilli(),
	}
}

// NotifyRestartComplete notifies the service that a server has completed its restart.
func (s *Service) NotifyRestartComplete(notification Notification) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.CurrentlyRestarting != notification.ServerName {
		s.log.Warn("Received restart completion from unexpected server",
			"expected", s.state.CurrentlyRestarting,
			"received", notification.ServerName)

		return fmt.Errorf("unexpected restart completion from server %s", notification.ServerName)
	}

	s.log.Info("Server restart completed",
		"name", notification.ServerName)

	// Clear currently restarting server
	s.state.CurrentlyRestarting = ""

	// Process next server in queue if any
	s.processQueue()

	return nil
}

// NotifyUnauthorizedRestart records that a server restarted without permission.
func (s *Service) NotifyUnauthorizedRestart(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.state.UnauthorizedRestarts[name] = append(s.state.UnauthorizedRestarts[name], now)
	s.state.RestartHistory[name] = now

	s.log.Warn("Server performed unauthorized restart",
		"name", name,
		"total_unauthorized", len(s.state.UnauthorizedRestarts[name]))

	// Remove from queue if present
	for i, entry := range s.state.Queue {
		if entry.ServerName == name {
			s.state.Queue = append(s.state.Queue[:i], s.state.Queue[i+1:]...)
			s.log.Debug("Removed server from queue due to unauthorized restart", "name", name)

			break
		}
	}

	// If this was the currently restarting server, clear it and process queue
	if s.state.CurrentlyRestarting == name {
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
	s.state.CurrentlyRestarting = nextServer.ServerName
	s.state.RestartHistory[nextServer.ServerName] = time.Now()

	s.log.Info("Processing next server from restart queue",
		"name", nextServer.ServerName,
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
					"name", entry.ServerName,
					"waitTime", now.Sub(entry.RequestTime))
			}
		}

		s.state.Queue = validQueue

		// Auto-clear stuck currently restarting server if it exceeds MaxRestartTime
		if s.state.CurrentlyRestarting != "" {
			name := s.state.CurrentlyRestarting

			// Determine when the restart started; if not found, remove the server
			startedAt := time.Time{}
			if v, exists := s.state.RestartHistory[name]; exists {
				startedAt = v
			}

			if startedAt.IsZero() || now.Sub(startedAt) >= s.config.MaxRestartTime {
				s.log.Warn("Currently restarting server exceeded max restart time; moving to next in queue",
					"name", name,
					"duration", now.Sub(startedAt),
					"maxRestartTime", s.config.MaxRestartTime)

				// Clear and process next in queue
				s.state.CurrentlyRestarting = ""
				s.processQueue()
			}
		}
		s.mu.Unlock()
	}
}

// GetState returns the current state of the restart manager (for debugging/monitoring).
func (s *Service) GetState() ServerState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid race conditions
	state := ServerState{
		CurrentlyRestarting:  s.state.CurrentlyRestarting,
		Queue:                make([]QueueEntry, len(s.state.Queue)),
		RestartHistory:       make(map[string]time.Time),
		UnauthorizedRestarts: make(map[string][]time.Time),
	}

	copy(state.Queue, s.state.Queue)
	maps.Copy(state.RestartHistory, s.state.RestartHistory)

	// Deep copy the UnauthorizedRestarts map of slices
	for name, times := range s.state.UnauthorizedRestarts {
		state.UnauthorizedRestarts[name] = make([]time.Time, len(times))
		copy(state.UnauthorizedRestarts[name], times)
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
