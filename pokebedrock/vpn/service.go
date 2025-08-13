package vpn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/internal"
)

// globalService ...
var globalService *Service

// GlobalService returns the global instance of the Service.
func GlobalService() *Service {
	return globalService
}

// Service is responsible for checking is a player connecting to the hub
// is on a vpn connection or not.
type Service struct {
	url    string
	closed atomic.Bool

	client *http.Client
	log    *slog.Logger

	rateLimitReset time.Time
	mu             sync.Mutex

	cache *Cache
}

// NewService initializes a new global service instance with the provided logger, URL.
func NewService(log *slog.Logger, url, cachePath string) {
	globalService = &Service{
		url: url,
		client: &http.Client{
			Timeout: requestTimeout,
		},
		log: log,
	}

	// Initialize cache (best-effort)
	if cachePath != "" {
		if c, err := NewCache(cachePath); err != nil {
			log.Warn("failed to initialize vpn cache", "error", err)
		} else {
			globalService.cache = c
		}
	}
}

const (
	maxRetries     = 1
	retryDelay     = 1 * time.Second
	requestTimeout = 1 * time.Second
)

// CheckIP determines whether the provided IP address is associated with a VPN connection.
func (s *Service) CheckIP(ip string) (*ResponseModel, error) {
	if net.ParseIP(ip) == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}

	// Fast path: cached result
	if s.cache != nil {
		if cached, ok := s.cache.Get(ip); ok {
			s.log.Info("VPN check result", "ip", ip, "proxy", cached)
			return &ResponseModel{Status: StatusSuccess, Proxy: cached}, nil
		}
	}

	s.mu.Lock()

	if time.Now().Before(s.rateLimitReset) {
		s.mu.Unlock()

		return nil, fmt.Errorf("rate limit active, please wait until %v", s.rateLimitReset)
	}
	s.mu.Unlock()

	var lastErr error

	for attempt := range maxRetries {
		if s.closed.Load() {
			return nil, fmt.Errorf("hub is shutting down")
		}

		if attempt > 0 {
			time.Sleep(retryDelay * time.Duration(1<<attempt))
		}

		url := fmt.Sprintf("%s/%s?fields=status,message,proxy", s.url, ip)
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			cancel()

			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		response, err := s.client.Do(request)

		cancel()

		if err != nil {
			if ErrorIsTemporary(err) {
				continue
			}

			lastErr = fmt.Errorf("request failed: %w", err)

			return nil, lastErr
		}

		defer response.Body.Close()

		s.handleRateLimitHeaders(response.Header)

		switch response.StatusCode {
		case http.StatusOK:
			var responseModel ResponseModel
			if err = json.NewDecoder(response.Body).Decode(&responseModel); err != nil {
				return nil, fmt.Errorf("failed to decode response body: %w", err)
			}

			if strings.EqualFold(responseModel.Status, "fail") {
				failMessage := responseModel.Message
				if strings.EqualFold(failMessage, "reserved range") {
					// Persist to cache
					if s.cache != nil {
						s.cache.Set(ip, false)
					}
					return &ResponseModel{Status: StatusSuccess, Proxy: false}, nil
				}

				return nil, fmt.Errorf("query failed: %s", failMessage)
			}

			// Persist to cache
			if s.cache != nil {
				s.cache.Set(ip, responseModel.Proxy)
			}

			return &responseModel, nil
		case http.StatusTooManyRequests:
			lastErr = fmt.Errorf("rate limited by api")

			time.Sleep(time.Duration(attempt+1) * retryDelay)

			continue
		default:
			lastErr = fmt.Errorf("unexpected status code: %d", response.StatusCode)
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("max retries reached")
	}

	return nil, lastErr
}

// handleRateLimitHeaders handles the rate limit headers.
func (s *Service) handleRateLimitHeaders(header http.Header) {
	requestsRemainingStr := header.Get("X-Rl")
	timeToResetStr := header.Get("X-Ttl")

	if requestsRemainingStr == "0" && timeToResetStr != "" {
		ttl, err := strconv.Atoi(timeToResetStr)
		if err != nil {
			// couldn't parse header for whatever reason, just default to fallback wait time.
			ttl = internal.DefaultTTL
		}

		s.mu.Lock()
		s.rateLimitReset = time.Now().Add(time.Duration(ttl) * time.Second)
		s.mu.Unlock()
		s.log.Warn("rate limit reached. waiting for reset.", "ttl_seconds", ttl)
	}
}

// Stop stops the service.
func (s *Service) Stop() {
	s.closed.Store(true)
}

// ErrorIsTemporary determines whether the given error is a temporary error that can be retried.
// This function checks for context deadline exceeded errors and network-related errors like timeouts.
func ErrorIsTemporary(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error

	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}
