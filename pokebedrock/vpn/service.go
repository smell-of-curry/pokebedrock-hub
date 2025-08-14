package vpn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
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

func (s *Service) CheckIP(ip string) (*ResponseModel, error) {
	if net.ParseIP(ip) == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}

	// Fast path: cached result
	if cached, found := s.checkCache(ip); found {
		return cached, nil
	}

	// Check rate limit
	if err := s.checkRateLimit(); err != nil {
		return nil, err
	}

	// Make API request
	return s.makeVPNRequest(ip)
}

// checkCache checks if the IP result is cached
func (s *Service) checkCache(ip string) (*ResponseModel, bool) {
	if s.cache != nil {
		if cached, ok := s.cache.Get(ip); ok {
			s.log.Info("VPN check result", "ip", ip, "proxy", cached)
			return &ResponseModel{Status: StatusSuccess, Proxy: cached}, true
		}
	}
	return nil, false
}

// checkRateLimit checks if we're currently rate limited
func (s *Service) checkRateLimit() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if time.Now().Before(s.rateLimitReset) {
		return fmt.Errorf("rate limit active, please wait until %v", s.rateLimitReset)
	}
	return nil
}

// makeVPNRequest makes the actual VPN API request
func (s *Service) makeVPNRequest(ip string) (*ResponseModel, error) {
	url := fmt.Sprintf("%s/%s?fields=status,message,proxy", s.url, ip)

	req, err := s.createRequest(url)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	response, err := s.executeRequest(req)
	if err != nil {
		return nil, err
	}

	result, err := s.processResponse(response, ip)
	if err != nil {
		return nil, err
	}

	// Cache the result if we have a cache
	s.cacheResult(ip, result.Proxy)

	return result, nil
}

// createRequest creates the HTTP request with proper headers
func (s *Service) createRequest(url string) (*http.Request, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// executeRequest executes the HTTP request with retry logic
func (s *Service) executeRequest(req *http.Request) (*http.Response, error) {
	const maxRetries = 3
	const retryDelay = 100 * time.Millisecond
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if s.closed.Load() {
			return nil, fmt.Errorf("service is shutting down")
		}

		if attempt > 0 {
			time.Sleep(retryDelay * time.Duration(1<<attempt))
		}

		response, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		if response.StatusCode == http.StatusOK {
			return response, nil
		}

		response.Body.Close()

		if err := s.handleHTTPError(response); err != nil {
			return nil, err
		}

		lastErr = fmt.Errorf("unexpected status: %d", response.StatusCode)
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// processResponse processes the HTTP response and parses the JSON
func (s *Service) processResponse(response *http.Response, ip string) (*ResponseModel, error) {
	defer response.Body.Close()

	var responseModel ResponseModel
	if err := json.NewDecoder(response.Body).Decode(&responseModel); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if responseModel.Status == "fail" {
		if responseModel.Message == "reserved range" {
			// Private IP range - not a proxy
			result := &ResponseModel{Status: StatusSuccess, Proxy: false}
			s.cacheResult(ip, false)
			return result, nil
		}
		return nil, fmt.Errorf("query failed: %s", responseModel.Message)
	}

	s.log.Info("VPN check result", "ip", ip, "proxy", responseModel.Proxy)
	return &responseModel, nil
}

// handleHTTPError handles specific HTTP error responses
func (s *Service) handleHTTPError(response *http.Response) error {
	switch response.StatusCode {
	case http.StatusTooManyRequests:
		return s.handleRateLimit(response)
	case http.StatusForbidden:
		return fmt.Errorf("API key invalid or insufficient permissions")
	case http.StatusBadRequest:
		return fmt.Errorf("bad request - invalid IP format")
	default:
		return nil // Continue retrying for other errors
	}
}

// handleRateLimit processes rate limit responses
func (s *Service) handleRateLimit(response *http.Response) error {
	resetHeader := response.Header.Get("X-RateLimit-Reset")
	if resetHeader != "" {
		if resetTime, err := time.Parse(time.RFC3339, resetHeader); err == nil {
			s.mu.Lock()
			s.rateLimitReset = resetTime
			s.mu.Unlock()
		}
	}
	return fmt.Errorf("rate limit exceeded")
}

// cacheResult caches the VPN check result
func (s *Service) cacheResult(ip string, isProxy bool) {
	if s.cache != nil {
		s.cache.Set(ip, isProxy)
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
