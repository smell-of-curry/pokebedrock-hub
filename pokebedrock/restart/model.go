// Package restart provides a model for the restart service.
package restart

import "time"

// Status represents the possible responses to a restart request.
type Status string

// Status constants for the restart service.
const (
	StatusAllow Status = "allow"
	StatusWait  Status = "wait"
	StatusDeny  Status = "deny"
)

// Request represents a request from a downstream server to restart.
type Request struct {
	ServerName string `json:"server_name" binding:"required"` // ex. BLACK
	Host       string `json:"host" binding:"required"`        // ex. 40.160.19.215:19136
}

// Response represents the hub's response to a restart request.
type Response struct {
	Status       Status `json:"status"`
	Message      string `json:"message,omitempty"`
	RetryAfter   int64  `json:"retry_after,omitempty"` // Unix timestamp when to retry
	QueuePos     int    `json:"queue_position,omitempty"`
	ResponseTime int64  `json:"response_time"`
}

// Notification represents a notification when a server has completed its restart.
type Notification struct {
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
	Status       Status
}

// ServerState represents the restart state of a server.
type ServerState struct {
	CurrentlyRestarting  string                 // Host that is currently restarting
	Queue                []QueueEntry           // Queue of servers waiting to restart
	RestartHistory       map[string]time.Time   // Last restart time for each server
	UnauthorizedRestarts map[string][]time.Time // All unauthorized restart times for each server
}
