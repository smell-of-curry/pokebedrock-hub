package rank

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/df-mc/dragonfly/server/player"
)

// globalService ...
var globalService *Service

// GlobalService ...
func GlobalService() *Service {
	return globalService
}

// Service ...
type Service struct {
	url string

	client *http.Client
	log    *slog.Logger
}

// NewService ...
func NewService(log *slog.Logger, url string) {
	globalService = &Service{
		url: url,
		client: &http.Client{
			Timeout: requestTimeout,
		},
		log: log,
	}
}

const (
	maxRetries     = 3
	retryDelay     = 1 * time.Second
	requestTimeout = 5 * time.Second
)

var (
	UserNotFound = fmt.Errorf("user not found")
	TimeoutError = fmt.Errorf("request timed out")
	ServerError  = fmt.Errorf("server error")
)

// RolesOfPlayer ...
func (s *Service) RolesOfPlayer(p *player.Player) ([]string, error) {
	return s.RolesOfXUID(p.XUID())
}

// RolesOfXUID ...
func (s *Service) RolesOfXUID(xuid string) ([]string, error) {
	var roles []string
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", s.url, xuid), nil)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := s.client.Do(req)
		cancel()
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			if isTemporaryError(err) {
				continue
			}
			return nil, lastErr
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				lastErr = fmt.Errorf("failed to read response: %w", err)
				continue
			}
			if err = json.Unmarshal(body, &roles); err != nil {
				return nil, fmt.Errorf("failed to parse roles: %w", err)
			}

			// Log that we successfully fetched the roles.
			s.log.Debug("Fetched roles", "xuid", xuid, "roles", roles)

			return roles, nil
		case http.StatusNotFound:
			return nil, UserNotFound
		case http.StatusTooManyRequests:
			lastErr = fmt.Errorf("rate limited")
			time.Sleep(time.Duration(attempt+1) * retryDelay)
			continue
		default:
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			if resp.StatusCode >= 500 {
				continue
			}

			return nil, fmt.Errorf("server returned %d: %w", resp.StatusCode, ServerError)
		}
	}
	return nil, lastErr
}

// isTemporaryError ...
func isTemporaryError(err error) bool {
	// Check for context deadline exceeded errors
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	return false
}

// RolesError parses a role error to be sent to a player.
func RolesError(err error) string {
	// TODO: Convert these to locale
	switch {
	case errors.Is(err, UserNotFound):
		return "Your account is not linked to the server."
	case errors.Is(err, TimeoutError):
		return "Timeout while fetching roles"
	case errors.Is(err, ServerError):
		return "Server error while fetching roles"
	default:
		return fmt.Sprintf("Failed to fetch roles %s", err)
	}
}
