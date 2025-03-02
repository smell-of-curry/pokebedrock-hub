package data

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Errors that can occur when fetching roles
var (
	UserNotFound = fmt.Errorf("user not found")
	TimeoutError = fmt.Errorf("request timed out")
	ServerError  = fmt.Errorf("server error")
)

const (
	roleAPIURL     = "http://15.204.44.68:4000/"
	requestTimeout = 5 * time.Second
	maxRetries     = 2
	retryDelay     = 500 * time.Millisecond
)

// Roles fetches the roles for a player with the given XUID.
// It includes retry logic for transient errors and timeouts.
func Roles(xuid string) ([]string, error) {
	var roles []string
	var lastErr error

	// Create a client with a timeout
	client := &http.Client{
		Timeout: requestTimeout,
	}

	// Try up to maxRetries times
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retrying
			time.Sleep(retryDelay)
		}

		// Make the request
		resp, err := client.Get(roleAPIURL + xuid)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			// Retry on timeout or temporary network errors
			if isTemporaryError(err) {
				continue
			}
			return nil, lastErr
		}

		// Process the response
		defer resp.Body.Close()

		// Handle different status codes
		switch resp.StatusCode {
		case http.StatusOK:
			// Success, parse the response
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				lastErr = fmt.Errorf("failed to read response: %w", err)
				continue
			}

			if err = json.Unmarshal(body, &roles); err != nil {
				return nil, fmt.Errorf("failed to parse roles: %w", err)
			}
			return roles, nil

		case http.StatusNotFound:
			return nil, UserNotFound

		case http.StatusTooManyRequests:
			// Rate limited, retry with exponential backoff
			lastErr = fmt.Errorf("rate limited")
			time.Sleep(time.Duration(attempt+1) * retryDelay)
			continue

		default:
			// Server error (5xx) or unexpected status code
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			if resp.StatusCode >= 500 {
				// Server errors are potentially recoverable, so retry
				continue
			}
			// Client errors (4xx) are not recoverable, so don't retry
			return nil, fmt.Errorf("server returned %d: %w", resp.StatusCode, ServerError)
		}
	}

	// If we get here, we've exhausted our retries
	return nil, lastErr
}

// isTemporaryError determines if an error is likely temporary and worth retrying
func isTemporaryError(err error) bool {
	if err == nil {
		return false
	}
	// Check for timeout errors
	if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
		return true
	}
	// Check for temporary network errors
	if netErr, ok := err.(interface{ Temporary() bool }); ok && netErr.Temporary() {
		return true
	}
	return false
}

// LogRolesError logs an error from the Roles function with appropriate context
func LogRolesError(log *slog.Logger, xuid string, err error) {
	if err == UserNotFound {
		log.Info("No roles found for player", "xuid", xuid)
	} else if err == TimeoutError {
		log.Warn("Timeout while fetching roles", "xuid", xuid)
	} else if err == ServerError {
		log.Error("Server error while fetching roles", "xuid", xuid)
	} else {
		log.Error("Failed to fetch roles", "xuid", xuid, "error", err)
	}
}
