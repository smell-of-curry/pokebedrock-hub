package moderation

import (
	"bytes"
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

	"github.com/df-mc/dragonfly/server/player"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/internal"
)

// globalService ...
var globalService *Service

// GlobalService returns the global service instance.
func GlobalService() *Service {
	return globalService
}

// Service represents a service for interacting with a moderation API.
// It holds the configuration for the service such as the URL, key, HTTP client, and logger.
type Service struct {
	url    string
	key    string
	closed bool

	client *http.Client
	log    *slog.Logger
}

// NewService initializes a new global service instance with the provided logger, URL, and authorization key.
// This function configures the HTTP client and sets up the service.
func NewService(log *slog.Logger, url, key string) {
	// Create a custom HTTP transport with optimized connection pooling
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: internal.LongOperationTimeoutSec * time.Second,
		}).DialContext,
		MaxIdleConns:          internal.DefaultChannelBufferSize,
		IdleConnTimeout:       3 * internal.LongOperationTimeoutSec * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   maxConcurrentRequests * 2, // Allow more idle connections per host
		MaxConnsPerHost:       maxConcurrentRequests * 3, // Limit max connections per host
	}

	globalService = &Service{
		url:    url,
		key:    key,
		closed: false,
		client: &http.Client{
			Timeout:   requestTimeout,
			Transport: transport,
		},
		log: log,
	}
}

const (
	maxRetries     = 1
	retryDelay     = internal.ShortRetryDelayMs * time.Millisecond
	requestTimeout = 2 * time.Second

	// Maximum number of concurrent API requests
	maxConcurrentRequests = 5
)

// InflictionOfPlayer retrieves the inflictions for a given player by their XUID.
// This function internally calls `InflictionOfXUID` with the player's XUID.
func (s *Service) InflictionOfPlayer(p *player.Player) (*ModelResponse, error) {
	return s.InflictionOfXUID(p.XUID())
}

// InflictionOfXUID retrieves the inflictions for a specific XUID.
// This function makes a request to the service and returns the player's current and past inflictions.
func (s *Service) InflictionOfXUID(xuid string) (*ModelResponse, error) {
	return s.InflictionOf(&ModelRequest{XUID: xuid})
}

// InflictionOfName retrieves the inflictions for a player based on their name.
// This function makes a request to the service and returns the player's current and past inflictions.
func (s *Service) InflictionOfName(name string) (*ModelResponse, error) {
	return s.InflictionOf(&ModelRequest{Name: name})
}

// InflictionOf makes a request to the service to retrieve the inflictions based on a given request model.
// It handles retries, timeouts, and different server response codes, including parsing the response.
func (s *Service) InflictionOf(req *ModelRequest) (*ModelResponse, error) {
	rawRequest, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if s.closed {
			break
		}

		if attempt > 0 {
			time.Sleep(retryDelay * time.Duration(1<<attempt))
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url+"/getInflictions", bytes.NewBuffer(rawRequest))
		if err != nil {
			cancel()

			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("authorization", s.key)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(httpReq)

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
			var response ModelResponse
			if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
				return nil, err
			}

			s.log.Debug(fmt.Sprintf("Fetched inflictions of xuid=%s,name=%s and response=%+v", req.XUID, req.Name, response))

			return &response, nil
		case http.StatusTooManyRequests:
			lastErr = fmt.Errorf("rate limited")

			time.Sleep(time.Duration(attempt+1) * retryDelay)

			continue
		default:
			body, _ := io.ReadAll(resp.Body)

			return nil, fmt.Errorf("failed to get inflictions: %s", string(body))
		}
	}

	return nil, lastErr
}

// AddInfliction adds a new infliction (e.g., ban, mute) to the player.
// It sends a request to the service and retries in case of temporary errors.
func (s *Service) AddInfliction(req *ModelRequest) error {
	rawRequest, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if s.closed {
			break
		}

		if attempt > 0 {
			time.Sleep(retryDelay * time.Duration(1<<attempt))
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url+"/addInfliction",
			bytes.NewBuffer(rawRequest))
		s.log.Debug(fmt.Sprintf("Adding infliction on url=%s,request=%+v",
			s.url+"/addInfliction", bytes.NewBuffer(rawRequest)))

		if err != nil {
			cancel()

			return fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("authorization", s.key)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(httpReq)

		cancel()

		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)

			if isTemporaryError(err) {
				continue
			}

			return lastErr
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent {
			s.log.Debug(fmt.Sprintf("Successfully added or updated infliction for xuid=%s,name=%s", req.XUID, req.Name))

			return nil
		}

		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("failed to add infliction: %s", string(body))
	}

	return lastErr
}

// RemoveInfliction removes an existing infliction (e.g., un-ban, un-mute) from a player.
// It sends a request to the service to remove the infliction and retries on temporary errors.
func (s *Service) RemoveInfliction(req *ModelRequest) error {
	rawRequest, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if s.closed {
			break
		}

		if attempt > 0 {
			time.Sleep(retryDelay * time.Duration(1<<attempt))
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, 
			s.url+"/removeInfliction", bytes.NewBuffer(rawRequest))
		if err != nil {
			cancel()

			return fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("authorization", s.key)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(httpReq)

		cancel()

		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)

			if isTemporaryError(err) {
				continue
			}

			return lastErr
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent {
			s.log.Debug(fmt.Sprintf("Successfully removed infliction for xuid=%s,name=%s", req.XUID, req.Name))

			return nil
		}

		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("failed to remove infliction: %s", string(body))
	}

	return lastErr
}

// SendDetailsOfQueue is a buffered channel for queueing player detail requests
var SendDetailsOfQueue = make(chan playerDetailsRequest, internal.DefaultChannelBufferSize)

// Used to signal worker shutdown
var detailsWorkerShutdown = make(chan struct{})

// playerDetailsRequest represents a queued request to send player details
type playerDetailsRequest struct {
	player *player.Player
}

// init starts the background worker for processing player details requests
func init() {
	go playerDetailsWorker()
}

// playerDetailsWorker processes queued player detail requests with rate limiting
func playerDetailsWorker() {
	// Create a semaphore using a buffered channel to limit concurrent requests
	semaphore := make(chan struct{}, maxConcurrentRequests)

	// Track active requests to ensure we can shut down cleanly
	activeRequests := make(chan struct{}, maxConcurrentRequests)

	for {
		select {
		case <-detailsWorkerShutdown:
			waitForActiveDetailsRequests(activeRequests)
			return
		case req, ok := <-SendDetailsOfQueue:
			if !ok {
				// Channel closed, exit worker
				return
			}

			if !tryProcessDetailsRequest(semaphore, activeRequests, req) {
				return // Worker is shutting down
			}
		}
	}
}

// waitForActiveDetailsRequests waits for all active requests to complete before shutdown
func waitForActiveDetailsRequests(activeRequests chan struct{}) {
	for range len(activeRequests) {
		<-activeRequests
	}
}

// tryProcessDetailsRequest attempts to process a player details request with concurrency limits
func tryProcessDetailsRequest(semaphore, activeRequests chan struct{}, req playerDetailsRequest) bool {
	// Acquire semaphore slot (blocks if maxConcurrentRequests are already running)
	select {
	case semaphore <- struct{}{}:
		// Track active request
		activeRequests <- struct{}{}

		// Process request in a goroutine
		go processPlayerDetailsRequest(semaphore, activeRequests, req.player)
		return true
	case <-detailsWorkerShutdown:
		// Worker is shutting down, don't start new requests
		return false
	}
}

// processPlayerDetailsRequest processes a single player details request
func processPlayerDetailsRequest(semaphore, activeRequests chan struct{}, p *player.Player) {
	defer func() {
		// Release semaphore slot when done
		<-semaphore
		// Mark request as complete
		<-activeRequests
	}()

	// Skip if service is closed
	s := GlobalService()
	if s == nil || s.closed {
		return
	}

	sendPlayerDetails(s, p)
}

// sendPlayerDetails sends player details to the moderation service
func sendPlayerDetails(s *Service, p *player.Player) {
	req := PlayerDetails{
		Name: p.Name(),
		XUID: p.XUID(),
		IP:   strings.Split(p.Addr().String(), ":")[0],
	}

	rawRequest, err := json.Marshal(req)
	if err != nil {
		s.log.Error(fmt.Sprintf("failed to marshal request: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, 
		s.url+"/playerDetails", bytes.NewBuffer(rawRequest))
	s.log.Debug(fmt.Sprintf("Sending details on url=%s,request=%+v", 
		s.url+"/playerDetails", bytes.NewBuffer(rawRequest)))

	if err != nil {
		s.log.Error(fmt.Sprintf("failed to create new request: %v", err))
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("authorization", s.key)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		s.log.Error(fmt.Sprintf("request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	s.log.Info(fmt.Sprintf("Sent player details of %s, status: %d", p.Name(), resp.StatusCode))
}

// SendDetailsOf queues a request to send player details to the API
func (s *Service) SendDetailsOf(p *player.Player) {
	if s.closed {
		return
	}

	// Queue the request instead of processing it immediately
	select {
	case SendDetailsOfQueue <- playerDetailsRequest{player: p}:
		// Successfully queued
	default:
		// Queue is full, log warning
		s.log.Error(fmt.Sprintf("Player details queue is full, skipping request for %s", p.Name()))
	}
}

// Stop stops the service and associated workers.
func (s *Service) Stop() {
	s.log.Debug("Stopping moderation service and workers...")
	s.closed = true

	// Signal the worker to shutdown
	close(detailsWorkerShutdown)

	// Give workers time to finish active requests (up to 3 seconds)
	timeout := time.NewTimer(3 * time.Second)
	<-timeout.C
}

// isTemporaryError checks if an error is temporary and can be retried.
// It checks for context deadline exceeded errors and network-related errors (e.g., timeout, temporary issues).
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
