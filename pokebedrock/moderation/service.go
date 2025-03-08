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
	key string

	client *http.Client
	log    *slog.Logger
}

// NewService ...
func NewService(log *slog.Logger, url, key string) {
	globalService = &Service{
		url: url,
		key: key,
		client: &http.Client{
			Timeout: requestTimeout,
		},
		log: log,
	}
}

const (
	maxRetries     = 3
	retryDelay     = 300 * time.Millisecond
	requestTimeout = 5 * time.Second
)

// InflictionOfPlayer ...
func (s *Service) InflictionOfPlayer(p *player.Player) (*ModelResponse, error) {
	return s.InflictionOfXUID(p.XUID())
}

// InflictionOfXUID ...
func (s *Service) InflictionOfXUID(xuid string) (*ModelResponse, error) {
	return s.InflictionOf(ModelRequest{XUID: xuid})
}

// InflictionOfName ...
func (s *Service) InflictionOfName(name string) (*ModelResponse, error) {
	return s.InflictionOf(ModelRequest{Name: name})
}

// InflictionOf ...
func (s *Service) InflictionOf(req ModelRequest) (*ModelResponse, error) {
	rawRequest, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url+"/getInflictions", bytes.NewBuffer(rawRequest))
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

// AddInfliction ...
func (s *Service) AddInfliction(req ModelRequest) error {
	rawRequest, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url+"/addInfliction", bytes.NewBuffer(rawRequest))
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

// RemoveInfliction ...
func (s *Service) RemoveInfliction(req ModelRequest) error {
	rawRequest, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.url+"/removeInfliction", bytes.NewBuffer(rawRequest))
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
