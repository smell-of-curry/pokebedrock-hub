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

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/internal"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
)

// globalService ...
var globalService *Service

// GlobalService returns the global instance of the Service.
func GlobalService() *Service {
	return globalService
}

// Service is responsible for interacting with the service that provides player roles.
// It contains configuration details like the service URL, HTTP client, and a logger for debugging.
type Service struct {
	url    string
	closed bool

	client *http.Client
	log    *slog.Logger
}

// NewService initializes a new global service instance with the given logger and service URL.
// This function sets up the service configuration, including the HTTP client and logger.
func NewService(log *slog.Logger, url string) {
	globalService = &Service{
		url:    url,
		closed: false,
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

// Error constants for the rank service.
var (
	ErrUserNotFound = fmt.Errorf("user not found")
	ErrTimeout      = fmt.Errorf("request timed out")
	ErrServer       = fmt.Errorf("server error")
)

// RolesOfPlayer retrieves the roles associated with the given player.
// This function delegates the request to the RolesOfXUID function using the player's XUID.
func (s *Service) RolesOfPlayer(p *player.Player) ([]string, error) {
	return s.RolesOfXUID(p.XUID())
}

// RolesOfXUID retrieves the roles associated with the specified XUID.
// It sends a request to the service's API and processes the response.
// The function retries on certain errors such as timeouts or temporary network issues.
func (s *Service) RolesOfXUID(xuid string) ([]string, error) {
	var roles []string

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if s.closed {
			break
		}

		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/discord/%s", s.url, xuid), nil)
		if err != nil {
			cancel()

			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := s.client.Do(req)

		cancel()

		if err != nil {
			if isTemporaryError(err) {
				continue
			}

			lastErr = fmt.Errorf("request failed: %w", err)

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
			return nil, ErrUserNotFound
		case http.StatusTooManyRequests:
			lastErr = fmt.Errorf("rate limited")

			time.Sleep(time.Duration(attempt+1) * retryDelay)

			continue
		default:
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)

			if resp.StatusCode >= internal.InternalServerError {
				continue
			}

			return nil, fmt.Errorf("server returned %d: %w", resp.StatusCode, ErrServer)
		}
	}

	return nil, lastErr
}

// Stop stops the service.
func (s *Service) Stop() {
	s.closed = true
}

// isTemporaryError determines whether the given error is a temporary error that can be retried.
// This function checks for context deadline exceeded errors and network-related errors like timeouts.
func isTemporaryError(err error) bool {
	// Check for context deadline exceeded errors
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}

// RolesError parses a role error to be sent to a player.
func RolesError(err error) string {
	switch {
	case errors.Is(err, ErrUserNotFound):
		return locale.Translate("error.account_not_linked")
	case errors.Is(err, ErrTimeout):
		return locale.Translate("error.timeout_fetching_roles")
	case errors.Is(err, ErrServer):
		return locale.Translate("error.server_error_fetching_roles")
	default:
		return fmt.Sprintf("Failed to fetch roles %s", err)
	}
}
