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
}

// Response represents the hub's response to a restart request.
type Response struct {
	Status     Status `json:"status"`
	Message    string `json:"message,omitempty"`
	RetryAfter int64  `json:"retry_after,omitempty"` // Unix timestamp when to retry
	QueuePos   int    `json:"queue_position,omitempty"`
}

// Notification represents a notification when a server has completed its restart.
type Notification struct {
	ServerName    string `json:"server_name" binding:"required"` // ex. BLACK
	CompletedTime int64  `json:"completed_time" binding:"required"`
}

// QueueEntry represents a server in the restart queue.
type QueueEntry struct {
	ServerName   string // ex. BLACK
	RequestTime  time.Time
	LastRetry    time.Time
	RetryCount   int
	FailureCount int
	Status       Status
}

// ServerState represents the restart state of a server.
type ServerState struct {
	CurrentlyRestarting  string                 // Name of the server that is currently restarting
	Queue                []QueueEntry           // Queue of servers waiting to restart
	RestartHistory       map[string]time.Time   // Last restart time for each server
	UnauthorizedRestarts map[string][]time.Time // All unauthorized restart times for each server
}
