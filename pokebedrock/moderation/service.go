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
	"sync"
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/player"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/internal"
)

// globalService holds the singleton moderation service.
var globalService *Service

// GlobalService returns the singleton moderation service.
func GlobalService() *Service {
	return globalService
}

// Service talks to the players-service moderation API. The closed flag is
// atomic so concurrent readers (worker goroutines, request callers) and the
// shutdown writer don't race.
type Service struct {
	url    string
	key    string
	closed atomic.Bool

	client *http.Client
	log    *slog.Logger
}

// NewService initialises the singleton moderation service.
// url should be the players service base URL (e.g. https://players.pokebedrock.com).
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
		url: strings.TrimRight(url, "/"),
		key: key,
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

	// maxConcurrentRequests bounds the player-details worker's parallelism.
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

// inflictionOf makes a GET request to /api/players/inflictions with the given
// query params and maps the players-service response to the internal model.
func (s *Service) inflictionOf(params url.Values) (*ModelResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if s.closed.Load() {
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

		out, retry, err := decodeInflictionsResponse(resp)
		if retry {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * retryDelay)
			continue
		}
		if err != nil {
			return nil, err
		}

		s.log.Debug(fmt.Sprintf("fetched inflictions via players service, active=%d inactive=%d",
			len(out.CurrentInflictions), len(out.PastInflictions)))
		return out, nil
	}

	return nil, lastErr
}

// decodeInflictionsResponse parses a GET /api/players/inflictions response.
// The returned retry flag indicates the caller should sleep and try again (for
// example, on rate limiting). A 404 is treated as "no inflictions".
func decodeInflictionsResponse(resp *http.Response) (out *ModelResponse, retry bool, err error) {
	defer closeBody(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		var apiResp apiInflictionsResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			return nil, false, err
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
		return result, false, nil
	case http.StatusNotFound:
		return &ModelResponse{}, false, nil
	case http.StatusTooManyRequests:
		return nil, true, fmt.Errorf("rate limited")
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("failed to get inflictions: %s", string(body))
	}
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
		if s.closed.Load() {
			break
		}
		if attempt > 0 {
			time.Sleep(retryDelay * time.Duration(1<<attempt))
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		endpoint := s.url + "/api/inflictions"
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(rawRequest))
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

		status := resp.StatusCode
		body, _ := io.ReadAll(resp.Body)
		closeBody(resp)

		if status == http.StatusCreated || status == http.StatusOK {
			// Log a bounded identifier only; never the full UserContext (PII).
			s.log.Debug("added infliction", "xuid", userCtx.XUID, "name", userCtx.Name)
			return nil
		}
		return fmt.Errorf("failed to add infliction: %s", string(body))
	}

	return lastErr
}

// RemoveInfliction removes an existing infliction by its UUID via the players service.
func (s *Service) RemoveInfliction(inflictionID string) error {
	if inflictionID == "" {
		return errors.New("infliction ID is required")
	}

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if s.closed.Load() {
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

		status := resp.StatusCode
		body, _ := io.ReadAll(resp.Body)
		closeBody(resp)

		if status == http.StatusNoContent || status == http.StatusOK {
			s.log.Debug("removed infliction", "id", inflictionID)
			return nil
		}
		return fmt.Errorf("failed to remove infliction: %s", string(body))
	}

	return lastErr
}

// closeBody drains and closes an HTTP response body so the connection can be
// reused by the keep-alive pool.
func closeBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// SendDetailsOfQueue is the buffered channel for queued player detail pushes.
var SendDetailsOfQueue = make(chan playerDetailsRequest, internal.DefaultChannelBufferSize)

// detailsWorkerShutdown signals the player-details worker to exit.
var detailsWorkerShutdown = make(chan struct{})

// detailsWorkerShutdownOnce guards detailsWorkerShutdown against double-close.
var detailsWorkerShutdownOnce sync.Once

// playerDetailsRequest represents a queued request to push player details.
type playerDetailsRequest struct {
	details PlayerDetails
}

func init() {
	go playerDetailsWorker()
}

// playerDetailsWorker processes queued player detail requests with a
// concurrency limit.
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

				go func(details PlayerDetails) {
					defer func() {
						<-semaphore
						<-activeRequests
					}()

					sendPlayerDetails(details)
				}(req.details)
			case <-detailsWorkerShutdown:
				return
			}
		}
	}
}

// sendPlayerDetails upserts a player into the players service via POST /api/players.
func sendPlayerDetails(req PlayerDetails) {
	s := GlobalService()
	if s == nil || s.closed.Load() {
		return
	}

	body := apiUpsertPlayer{
		XUID: req.XUID,
		Name: req.Name,
	}
	if req.IP != "" {
		body.IPs = []string{req.IP}
	}

	rawRequest, err := json.Marshal(body)
	if err != nil {
		s.log.Error("failed to marshal player details", "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	endpoint := s.url + "/api/players"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(rawRequest))
	if err != nil {
		s.log.Error("failed to create player details request", "error", err)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.setAuthHeaders(httpReq)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		s.log.Error("player details request failed", "error", err)
		return
	}
	defer closeBody(resp)

	s.log.Info("sent player details", "name", req.Name, "status", resp.StatusCode)
}

// hostFromAddress extracts the host portion of a host:port string, correctly
// handling IPv6 addresses (net.SplitHostPort) rather than a naive ":" split.
func hostFromAddress(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	return host
}

// SendDetailsOf enqueues a player-details push. Captures identity on the world
// owner so the HTTP worker never touches a live *player.Player.
func (s *Service) SendDetailsOf(p *player.Player) {
	if s.closed.Load() {
		return
	}

	details := PlayerDetails{
		Name: p.Name(),
		XUID: p.XUID(),
		IP:   hostFromAddress(p.Addr().String()),
	}

	select {
	case SendDetailsOfQueue <- playerDetailsRequest{details: details}:
	default:
		s.log.Error("player details queue is full, skipping request", "name", details.Name)
	}
}

// Stop signals the service and worker to shut down. The call blocks for up to
// 3 seconds while in-flight requests drain.
func (s *Service) Stop() {
	s.log.Debug("Stopping moderation service and workers...")
	s.closed.Store(true)

	detailsWorkerShutdownOnce.Do(func() {
		close(detailsWorkerShutdown)
	})

	<-time.After(3 * time.Second)
}

// isTemporaryError reports whether the error is one we should retry.
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
