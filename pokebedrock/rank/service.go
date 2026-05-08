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
	"strings"
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/player"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/internal"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
)

// globalService holds the singleton rank service.
var globalService *Service

// GlobalService returns the singleton rank service.
func GlobalService() *Service {
	return globalService
}

// Service fetches roles for a player from the upstream rank API.
//
// The closed flag is atomic so concurrent readers (request callers) and
// the shutdown writer don't race.
type Service struct {
	url    string
	closed atomic.Bool

	client *http.Client
	log    *slog.Logger
}

// NewService initialises the singleton rank service.
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

// Public sentinel errors returned by RolesOfXUID.
var (
	ErrUserNotFound = fmt.Errorf("user not found")
	ErrTimeout      = fmt.Errorf("request timed out")
	ErrServer       = fmt.Errorf("server error")
)

// RolesOfPlayer fetches the roles for the given player.
func (s *Service) RolesOfPlayer(p *player.Player) ([]string, error) {
	return s.RolesOfXUID(p.XUID())
}

// RolesOfXUID fetches the roles for the given XUID, retrying transient
// failures.
func (s *Service) RolesOfXUID(xuid string) ([]string, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if s.closed.Load() {
			break
		}
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		roles, retry, err := s.fetchRoles(xuid, attempt)
		if err == nil {
			return roles, nil
		}

		lastErr = err
		if retry {
			continue
		}

		return nil, err
	}

	return nil, lastErr
}

// fetchRoles performs a single attempt at fetching roles. The retry flag
// indicates the caller should sleep and try again.
func (s *Service) fetchRoles(xuid string, attempt int) (roles []string, retry bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/discord/%s", s.url, xuid), nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		if isTemporaryError(err) {
			return nil, true, err
		}

		return nil, false, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, true, fmt.Errorf("failed to read response: %w", err)
		}
		if err := json.Unmarshal(body, &roles); err != nil {
			return nil, false, fmt.Errorf("failed to parse roles: %w", err)
		}

		s.log.Debug("fetched roles", "xuid", xuid, "roles", roles)

		return roles, false, nil
	case http.StatusNotFound:
		return nil, false, ErrUserNotFound
	case http.StatusTooManyRequests:
		time.Sleep(time.Duration(attempt+1) * retryDelay)

		return nil, true, fmt.Errorf("rate limited")
	default:
		if resp.StatusCode >= internal.InternalServerError {
			return nil, true, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		return nil, false, fmt.Errorf("server returned %d: %w", resp.StatusCode, ErrServer)
	}
}

// Stop signals the service to stop accepting new requests. In-flight
// requests still run to completion.
func (s *Service) Stop() {
	s.closed.Store(true)
}

// isTemporaryError reports whether the error is transient and worth
// retrying.
func isTemporaryError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}

// RolesError converts an error from RolesOfXUID into a user-facing message.
func RolesError(err error) string {
	switch {
	case errors.Is(err, ErrUserNotFound):
		return locale.Translate("error.account_not_linked")
	case errors.Is(err, ErrTimeout):
		return locale.Translate("error.timeout_fetching_roles")
	case errors.Is(err, ErrServer) || strings.Contains(err.Error(), "actively refused"):
		return locale.Translate("error.server_error_fetching_roles")
	default:
		return fmt.Sprintf("Failed to fetch roles %s", err)
	}
}
