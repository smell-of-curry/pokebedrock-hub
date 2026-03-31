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
	"net/url"
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

// Service represents a service for interacting with the players-service moderation API.
// It holds the configuration for the service such as the URL, key, HTTP client, and logger.
type Service struct {
	url    string
	key    string
	closed bool

	client *http.Client
	log    *slog.Logger
}

// NewService initializes a new global service instance with the provided logger, URL, and authorization key.
// url should be the players service base URL (e.g. http://players:4002).
// key is the x-api-key value for authenticating with the players service.
func NewService(log *slog.Logger, url, key string) {
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
		MaxIdleConnsPerHost:   maxConcurrentRequests * 2,
		MaxConnsPerHost:       maxConcurrentRequests * 3,
	}

	globalService = &Service{
		url:    strings.TrimRight(url, "/"),
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

	maxConcurrentRequests = 5
)

// setAuthHeaders sets the x-api-key header used by the players service.
func (s *Service) setAuthHeaders(req *http.Request) {
	req.Header.Set("x-api-key", s.key)
}

// InflictionOfPlayer retrieves the inflictions for a given player by their XUID.
func (s *Service) InflictionOfPlayer(p *player.Player) (*ModelResponse, error) {
	return s.InflictionOfXUID(p.XUID())
}

// InflictionOfXUID retrieves the inflictions for a specific XUID.
func (s *Service) InflictionOfXUID(xuid string) (*ModelResponse, error) {
	params := url.Values{}
	params.Set("xuid", xuid)
	return s.inflictionOf(params)
}

// InflictionOfName retrieves the inflictions for a player based on their name.
func (s *Service) InflictionOfName(name string) (*ModelResponse, error) {
	params := url.Values{}
	params.Set("name", name)
	return s.inflictionOf(params)
}

// inflictionOf makes a GET request to /api/players/inflictions with the given query params.
func (s *Service) inflictionOf(params url.Values) (*ModelResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if s.closed {
			break
		}

		if attempt > 0 {
			time.Sleep(retryDelay * time.Duration(1<<attempt))
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)

		endpoint := s.url + "/api/players/inflictions?" + params.Encode()
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		s.setAuthHeaders(httpReq)

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
			var apiResp apiInflictionsResponse
			if err = json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				return nil, err
			}

			result := &ModelResponse{
				CurrentInflictions: make([]Infliction, 0, len(apiResp.Active)),
				PastInflictions:    make([]Infliction, 0, len(apiResp.Inactive)),
			}
			for _, a := range apiResp.Active {
				result.CurrentInflictions = append(result.CurrentInflictions, apiInflictionToInternal(a))
			}
			for _, a := range apiResp.Inactive {
				result.PastInflictions = append(result.PastInflictions, apiInflictionToInternal(a))
			}

			s.log.Debug(fmt.Sprintf("Fetched inflictions via players service, active=%d inactive=%d", len(apiResp.Active), len(apiResp.Inactive)))

			return result, nil
		case http.StatusTooManyRequests:
			lastErr = fmt.Errorf("rate limited")
			time.Sleep(time.Duration(attempt+1) * retryDelay)
			continue
		case http.StatusNotFound:
			return &ModelResponse{}, nil
		default:
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("failed to get inflictions: %s", string(body))
		}
	}

	return nil, lastErr
}

// AddInfliction creates a new infliction via the players service.
// userCtx identifies the target player; infliction holds the punishment details.
func (s *Service) AddInfliction(userCtx UserContext, infliction Infliction) error {
	apiReq := apiCreateRequest{
		UserContext: userCtx,
		Infliction:  internalToAPICreate(infliction),
	}

	rawRequest, err := json.Marshal(apiReq)
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
		endpoint := s.url + "/api/inflictions"
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(rawRequest))
		s.log.Debug(fmt.Sprintf("Adding infliction on url=%s", endpoint))

		if err != nil {
			cancel()
			return fmt.Errorf("failed to create request: %w", err)
		}

		s.setAuthHeaders(httpReq)
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

		if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
			s.log.Debug(fmt.Sprintf("Successfully added infliction for %+v", userCtx))
			return nil
		}

		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add infliction: %s", string(body))
	}

	return lastErr
}

// RemoveInfliction removes an existing infliction by its UUID via the players service.
func (s *Service) RemoveInfliction(inflictionID string) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if s.closed {
			break
		}

		if attempt > 0 {
			time.Sleep(retryDelay * time.Duration(1<<attempt))
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		endpoint := s.url + "/api/inflictions/" + inflictionID
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
		if err != nil {
			cancel()
			return fmt.Errorf("failed to create request: %w", err)
		}

		s.setAuthHeaders(httpReq)

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

		if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
			s.log.Debug(fmt.Sprintf("Successfully removed infliction id=%s", inflictionID))
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
	semaphore := make(chan struct{}, maxConcurrentRequests)
	activeRequests := make(chan struct{}, maxConcurrentRequests)

	for {
		select {
		case <-detailsWorkerShutdown:
			for range len(activeRequests) {
				<-activeRequests
			}
			return
		case req, ok := <-SendDetailsOfQueue:
			if !ok {
				return
			}

			select {
			case semaphore <- struct{}{}:
				activeRequests <- struct{}{}

				go func(p *player.Player) {
					defer func() {
						<-semaphore
						<-activeRequests
					}()

					s := GlobalService()
					if s == nil || s.closed {
						return
					}

					body := apiUpsertPlayer{
						XUID: p.XUID(),
						Name: p.Name(),
						IPs:  []string{strings.Split(p.Addr().String(), ":")[0]},
					}

					rawRequest, err := json.Marshal(body)
					if err != nil {
						s.log.Error(fmt.Sprintf("failed to marshal request: %v", err))
						return
					}

					ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
					defer cancel()

					endpoint := s.url + "/api/players"
					httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(rawRequest))
					s.log.Debug(fmt.Sprintf("Sending player details to %s", endpoint))

					if err != nil {
						s.log.Error(fmt.Sprintf("failed to create new request: %v", err))
						return
					}

					httpReq.Header.Set("Content-Type", "application/json")
					s.setAuthHeaders(httpReq)

					resp, err := s.client.Do(httpReq)
					if err != nil {
						s.log.Error(fmt.Sprintf("request failed: %v", err))
						return
					}
					defer resp.Body.Close()

					s.log.Info(fmt.Sprintf("Sent player details of %s, status: %d", p.Name(), resp.StatusCode))
				}(req.player)
			case <-detailsWorkerShutdown:
				return
			}
		}
	}
}

// SendDetailsOf queues a request to send player details to the players service (upsert).
func (s *Service) SendDetailsOf(p *player.Player) {
	if s.closed {
		return
	}

	select {
	case SendDetailsOfQueue <- playerDetailsRequest{player: p}:
	default:
		s.log.Error(fmt.Sprintf("Player details queue is full, skipping request for %s", p.Name()))
	}
}

// Stop stops the service and associated workers.
func (s *Service) Stop() {
	s.log.Debug("Stopping moderation service and workers...")
	s.closed = true
	close(detailsWorkerShutdown)
	timeout := time.NewTimer(3 * time.Second)
	<-timeout.C
}

// isTemporaryError checks if an error is temporary and can be retried.
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
