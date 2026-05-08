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

// Service talks to the moderation HTTP API. The closed flag is atomic so
// concurrent readers (worker goroutines, request callers) and the
// shutdown writer don't race.
type Service struct {
	url    string
	key    string
	closed atomic.Bool

	client *http.Client
	log    *slog.Logger
}

// NewService initialises the singleton moderation service.
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
		url: url,
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

// InflictionOfPlayer fetches the inflictions for the given player.
func (s *Service) InflictionOfPlayer(p *player.Player) (*ModelResponse, error) {
	return s.InflictionOfXUID(p.XUID())
}

// InflictionOfXUID fetches the inflictions for the given XUID.
func (s *Service) InflictionOfXUID(xuid string) (*ModelResponse, error) {
	return s.InflictionOf(ModelRequest{XUID: xuid})
}

// InflictionOfName fetches the inflictions for the given player name.
func (s *Service) InflictionOfName(name string) (*ModelResponse, error) {
	return s.InflictionOf(ModelRequest{Name: name})
}

// InflictionOf retrieves the current and past inflictions matching the given request.
func (s *Service) InflictionOf(req ModelRequest) (*ModelResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if s.closed.Load() {
			break
		}
		if attempt > 0 {
			time.Sleep(retryDelay * time.Duration(1<<attempt))
		}

		resp, err := s.attempt(http.MethodPost, "/getInflictions", body)
		if err != nil {
			lastErr = err
			if isTemporaryError(err) {
				continue
			}

			return nil, err
		}

		out, retry, err := decodeInflictionsResponse(resp, req)
		if retry {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * retryDelay)

			continue
		}
		if err != nil {
			return nil, err
		}

		s.log.Debug("fetched inflictions", "xuid", req.XUID, "name", req.Name, "current", len(out.CurrentInflictions), "past", len(out.PastInflictions))

		return out, nil
	}

	return nil, lastErr
}

// AddInfliction submits a new infliction.
func (s *Service) AddInfliction(req ModelRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	s.log.Debug("adding infliction", "url", s.url+"/addInfliction", "xuid", req.XUID, "name", req.Name)

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if s.closed.Load() {
			break
		}
		if attempt > 0 {
			time.Sleep(retryDelay * time.Duration(1<<attempt))
		}

		resp, err := s.attempt(http.MethodPost, "/addInfliction", body)
		if err != nil {
			lastErr = err
			if isTemporaryError(err) {
				continue
			}

			return err
		}

		err = decodeNoContentResponse(resp, "add infliction")
		closeBody(resp)
		if err == nil {
			s.log.Debug("added infliction", "xuid", req.XUID, "name", req.Name)

			return nil
		}

		return err
	}

	return lastErr
}

// RemoveInfliction removes an existing infliction (un-ban, un-mute, etc.).
func (s *Service) RemoveInfliction(req ModelRequest) error {
	body, err := json.Marshal(req)
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

		resp, err := s.attempt(http.MethodDelete, "/removeInfliction", body)
		if err != nil {
			lastErr = err
			if isTemporaryError(err) {
				continue
			}

			return err
		}

		err = decodeNoContentResponse(resp, "remove infliction")
		closeBody(resp)
		if err == nil {
			s.log.Debug("removed infliction", "xuid", req.XUID, "name", req.Name)

			return nil
		}

		return err
	}

	return lastErr
}

// attempt issues a single HTTP request against the moderation API. The
// caller is responsible for closing the response body once it has been
// drained (use closeBody if no further processing is needed, otherwise
// close it manually).
func (s *Service) attempt(method, path string, body []byte) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)

	httpReq, err := http.NewRequestWithContext(ctx, method, s.url+path, bytes.NewReader(body))
	if err != nil {
		cancel()

		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("authorization", s.key)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		cancel()

		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Cancel only fires after the body is closed; defer it on the
	// response by wrapping the body.
	resp.Body = ctxCloser{ReadCloser: resp.Body, cancel: cancel}

	return resp, nil
}

// ctxCloser ensures we cancel the request context the moment the body is
// closed, releasing the associated timer slot.
type ctxCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c ctxCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()

	return err
}

func closeBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// decodeInflictionsResponse parses an InflictionOf response. The returned
// retry flag indicates the caller should sleep and try again (for example,
// on rate limiting).
func decodeInflictionsResponse(resp *http.Response, _ ModelRequest) (out *ModelResponse, retry bool, err error) {
	defer closeBody(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		var response ModelResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, false, err
		}

		return &response, false, nil
	case http.StatusTooManyRequests:
		return nil, true, fmt.Errorf("rate limited")
	default:
		body, _ := io.ReadAll(resp.Body)

		return nil, false, fmt.Errorf("failed to get inflictions: %s", string(body))
	}
}

// decodeNoContentResponse asserts the response has 204 No Content,
// returning a descriptive error otherwise.
func decodeNoContentResponse(resp *http.Response, what string) error {
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)

	return fmt.Errorf("failed to %s: %s", what, string(body))
}

// SendDetailsOfQueue is the buffered channel for queued player detail
// pushes.
var SendDetailsOfQueue = make(chan playerDetailsRequest, internal.DefaultChannelBufferSize)

// detailsWorkerShutdown signals the player-details worker to exit.
var detailsWorkerShutdown = make(chan struct{})

// detailsWorkerShutdownOnce guards detailsWorkerShutdown against double-close.
var detailsWorkerShutdownOnce sync.Once

// playerDetailsRequest represents a queued request to push player details.
type playerDetailsRequest struct {
	player *player.Player
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

				go func(p *player.Player) {
					defer func() {
						<-semaphore
						<-activeRequests
					}()

					sendPlayerDetails(p)
				}(req.player)
			case <-detailsWorkerShutdown:
				return
			}
		}
	}
}

func sendPlayerDetails(p *player.Player) {
	s := GlobalService()
	if s == nil || s.closed.Load() {
		return
	}

	req := PlayerDetails{
		Name: p.Name(),
		XUID: p.XUID(),
		IP:   strings.Split(p.Addr().String(), ":")[0],
	}

	body, err := json.Marshal(req)
	if err != nil {
		s.log.Error("failed to marshal player details", "error", err)

		return
	}

	s.log.Debug("sending player details", "url", s.url+"/playerDetails", "name", req.Name, "xuid", req.XUID)

	resp, err := s.attempt(http.MethodPost, "/playerDetails", body)
	if err != nil {
		s.log.Error("player details request failed", "error", err)

		return
	}
	defer closeBody(resp)

	s.log.Info("sent player details", "name", p.Name(), "status", resp.StatusCode)
}

// SendDetailsOf enqueues a player-details push.
func (s *Service) SendDetailsOf(p *player.Player) {
	if s.closed.Load() {
		return
	}

	select {
	case SendDetailsOfQueue <- playerDetailsRequest{player: p}:
	default:
		s.log.Error("player details queue is full, skipping request", "name", p.Name())
	}
}

// Stop signals the service and worker to shut down. The call blocks for up
// to 3 seconds while in-flight requests drain.
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
