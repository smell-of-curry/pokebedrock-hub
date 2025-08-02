// Package restart provides a model for the restart service.
package restart

import "time"

// RestartStatus represents the possible responses to a restart request.
type RestartStatus string

const (
	RestartStatusAllow RestartStatus = "allow"
	RestartStatusWait  RestartStatus = "wait"
	RestartStatusDeny  RestartStatus = "deny"
)

// RestartRequest represents a request from a downstream server to restart.
type RestartRequest struct {
	ServerName string `json:"server_name" binding:"required"` // ex. BLACK
	Host       string `json:"host" binding:"required"`        // ex. 40.160.19.215:19136
}

// RestartResponse represents the hub's response to a restart request.
type RestartResponse struct {
	Status       RestartStatus `json:"status"`
	Message      string        `json:"message,omitempty"`
	RetryAfter   int64         `json:"retry_after,omitempty"` // Unix timestamp when to retry
	QueuePos     int           `json:"queue_position,omitempty"`
	ResponseTime int64         `json:"response_time"`
}

// RestartNotification represents a notification when a server has completed its restart.
type RestartNotification struct {
	Host          string `json:"host" binding:"required"` // ex. 40.160.19.215:19136
	RestartTime   int64  `json:"restart_time" binding:"required"`
	CompletedTime int64  `json:"completed_time" binding:"required"`
}

// QueueEntry represents a server in the restart queue.
type QueueEntry struct {
	Host         string
	ServerName   string
	RequestTime  time.Time
	LastRetry    time.Time
	RetryCount   int
	FailureCount int
	Status       RestartStatus
}

// ServerRestartState represents the restart state of a server.
type ServerRestartState struct {
	CurrentlyRestarting  string                 // Host that is currently restarting
	Queue                []QueueEntry           // Queue of servers waiting to restart
	RestartHistory       map[string]time.Time   // Last restart time for each server
	UnauthorizedRestarts map[string][]time.Time // All unauthorized restart times for each server
}
