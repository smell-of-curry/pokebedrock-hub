// Package session provides a session for the server.
package session

import (
	"sync"
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/internal"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/moderation"
)

// Inflictions represents a player's inflictions like being muted or frozen.
type Inflictions struct {
	muted        atomic.Bool
	muteDuration atomic.Value[int64]
	frozen       atomic.Bool
}

// NewInflictions creates a new Inflictions object with default values.
func NewInflictions() *Inflictions {
	i := &Inflictions{}
	i.muted.Store(false)
	i.muteDuration.Store(0)
	i.frozen.Store(false)

	return i
}

// Maximum number of concurrent infliction requests
const maxConcurrentInflictionRequests = 5

// InflictionQueue is a buffered channel for queueing infliction load requests
var InflictionQueue = make(chan inflictionRequest, internal.DefaultChannelBufferSize)

// Used to signal worker shutdown
var inflictionWorkerShutdown = make(chan struct{})

// inflictionRequest represents a queued request to load player inflictions
type inflictionRequest struct {
	xuid        string
	handle      *world.EntityHandle
	inflictions *Inflictions
}

var (
	// inflictionWorkerWG counts running infliction workers so shutdown
	// can wait deterministically rather than relying on a fixed timer.
	inflictionWorkerWG sync.WaitGroup

	// inflictionShutdownOnce guards the shutdown channels against
	// double-close on repeated Stop calls.
	inflictionShutdownOnce sync.Once
)

func init() {
	inflictionWorkerWG.Add(1)
	go inflictionWorker()
}

// StopInflictionWorker stops all infliction workers gracefully and waits
// (with a hard cap of 3 seconds) for in-flight requests to finish.
func StopInflictionWorker() {
	inflictionShutdownOnce.Do(func() {
		close(inflictionWorkerShutdown)
	})

	done := make(chan struct{})
	go func() {
		inflictionWorkerWG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}

// inflictionWorker processes queued infliction load requests with rate
// limiting.
//
// The HTTP request to the moderation service is intentionally performed on
// this worker goroutine (NOT inside a world-owner callback). Running blocking
// HTTP calls on the owner serialises them and stalls world ticks. player.Do
// is used only for applying resulting inflictions.
func inflictionWorker() {
	defer inflictionWorkerWG.Done()

	semaphore := make(chan struct{}, maxConcurrentInflictionRequests)

	var activeRequestsWG sync.WaitGroup
	defer activeRequestsWG.Wait()

	for {
		select {
		case <-inflictionWorkerShutdown:
			return
		case req, ok := <-InflictionQueue:
			if !ok {
				return
			}

			select {
			case semaphore <- struct{}{}:
				activeRequestsWG.Add(1)

				go func(req inflictionRequest) {
					defer activeRequestsWG.Done()
					defer func() { <-semaphore }()

					processInflictionRequest(req)
				}(req)
			case <-inflictionWorkerShutdown:
				return
			}
		}
	}
}

// processInflictionRequest fetches inflictions for a player and applies them.
func processInflictionRequest(req inflictionRequest) {
	if req.handle == nil || req.xuid == "" || req.inflictions == nil {
		return
	}

	modSvc := moderation.GlobalService()
	if modSvc == nil {
		return
	}

	resp, err := modSvc.InflictionOfXUID(req.xuid)
	if err != nil || resp == nil {
		return
	}

	for _, inf := range resp.CurrentInflictions {
		switch inf.Type {
		case moderation.InflictionMuted:
			if inf.ExpiryDate != nil && *inf.ExpiryDate != 0 {
				req.inflictions.muteDuration.Store(*inf.ExpiryDate)
			}

			req.inflictions.muted.Store(true)
		case moderation.InflictionFrozen:
			req.inflictions.frozen.Store(true)
		}
	}

	player.Do(req.handle, func(_ *world.Tx, p *player.Player) {
		req.inflictions.handleActiveInflictions(p)
	})
}

// Load queues a request to load the player's inflictions from the moderation service.
//
// @param xuid Player XUID captured on the world owner before scheduling.
// @param handle Stable entity handle used to re-enter the player after HTTP.
func (i *Inflictions) Load(xuid string, handle *world.EntityHandle) {
	select {
	case InflictionQueue <- inflictionRequest{
		xuid:        xuid,
		handle:      handle,
		inflictions: i,
	}:
	default:
	}
}

// handleActiveInflictions applies the effects of active inflictions on the player.
func (i *Inflictions) handleActiveInflictions(p *player.Player) {
	if i.Frozen() {
		p.SetImmobile()
	}
}

// SetMuted sets whether the player is muted or not.
func (i *Inflictions) SetMuted(muted bool) {
	i.muted.Store(muted)
}

// Muted returns whether the player is muted or not.
func (i *Inflictions) Muted() bool {
	return i.muted.Load()
}

// SetMuteDuration sets the mute duration for the player.
func (i *Inflictions) SetMuteDuration(duration int64) {
	i.muteDuration.Store(duration)
}

// MuteDuration returns the current mute duration.
func (i *Inflictions) MuteDuration() int64 {
	return i.muteDuration.Load()
}

// SetFrozen sets whether the player is frozen or not.
func (i *Inflictions) SetFrozen(frozen bool) {
	i.frozen.Store(frozen)
}

// Frozen returns whether the player is frozen or not.
func (i *Inflictions) Frozen() bool {
	return i.frozen.Load()
}
