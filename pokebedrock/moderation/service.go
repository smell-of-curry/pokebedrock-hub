package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	key string

	client *http.Client
	log    *slog.Logger
}

// NewService ...
func NewService(log *slog.Logger, key string) {
	globalService = &Service{
		key: key,
		client: &http.Client{
			Timeout: requestTimeout,
		},
		log: log,
	}
}

// TODO: Add to config
const (
	baseURL = "https://pokebedrock.com/api/moderation"

	maxRetries     = 1
	retryDelay     = 300 * time.Millisecond
	requestTimeout = 2 * time.Second
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
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/getInflictions", baseURL), bytes.NewBuffer(rawRequest))
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
			continue
		}

		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK { // TODO: handle more status codes?
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("failed to get inflictions: %s", string(body))
		}

		var response ModelResponse
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, err
		}

		// Log that we got the response
		s.log.Debug(fmt.Sprintf("Fetched inflictions of %s, and got: %+v", req.XUID, response))

		return &response, nil
	}
	return nil, lastErr
}
