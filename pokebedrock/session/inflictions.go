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
	handle      *world.EntityHandle
	inflictions *Inflictions
}

var (
	// inflictionLoadQueue is a buffered channel for queuing rate-limited
	// infliction loads.
	inflictionLoadQueue = make(chan inflictionLoadRequest, internal.SmallChannelBufferSize)
	// inflictionLoadWorkerShutdown signals the loader to exit.
	inflictionLoadWorkerShutdown = make(chan struct{})

	// inflictionWorkerWG counts running infliction workers so shutdown
	// can wait deterministically rather than relying on a fixed timer.
	inflictionWorkerWG sync.WaitGroup

	// inflictionShutdownOnce guards the shutdown channels against
	// double-close on repeated Stop calls.
	inflictionShutdownOnce sync.Once
)

// inflictionLoadRequest represents a queued request to load player inflictions.
type inflictionLoadRequest struct {
	handle *world.EntityHandle
	inf    *Inflictions
}

func init() {
	inflictionWorkerWG.Add(2)
	go inflictionWorker()
	go inflictionLoadWorker()
}

// StopInflictionWorker stops all infliction workers gracefully and waits
// (with a hard cap of 3 seconds) for in-flight requests to finish.
func StopInflictionWorker() {
	inflictionShutdownOnce.Do(func() {
		close(inflictionWorkerShutdown)
		close(inflictionLoadWorkerShutdown)
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
// this worker goroutine (NOT inside an ExecWorld callback). Running blocking
// HTTP calls inside ExecWorld serialises them on the world's transaction
// goroutine, stalling world ticks and has been observed to crash the runtime
// under load. ExecWorld is used only for cheap reads (capturing the XUID)
// and writes (applying the resulting inflictions).
func inflictionWorker() {
	defer inflictionWorkerWG.Done()

	semaphore := make(chan struct{}, maxConcurrentInflictionRequests)

	// activeRequestsWG tracks per-request goroutines so shutdown can wait
	// for them to drain. A WaitGroup is preferable to the previous
	// fixed-capacity channel which could deadlock if the slot count
	// drifted from the in-flight count.
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

				go func(handle *world.EntityHandle, inflictions *Inflictions) {
					defer activeRequestsWG.Done()
					defer func() { <-semaphore }()

					processInflictionRequest(handle, inflictions)
				}(req.handle, req.inflictions)
			case <-inflictionWorkerShutdown:
				return
			}
		}
	}
}

// processInflictionRequest fetches inflictions for a player and applies them.
func processInflictionRequest(handle *world.EntityHandle, inflictions *Inflictions) {
	if handle == nil {
		return
	}

	modSvc := moderation.GlobalService()
	if modSvc == nil {
		return
	}

	// Capture the XUID via a quick ExecWorld read.
	var xuid string
	handle.ExecWorld(func(_ *world.Tx, e world.Entity) {
		if p, ok := e.(*player.Player); ok {
			xuid = p.XUID()
		}
	})

	if xuid == "" {
		// Player is no longer in the world.
		return
	}

	resp, err := modSvc.InflictionOfXUID(xuid)
	if err != nil || resp == nil {
		return
	}

	for _, inf := range resp.CurrentInflictions {
		switch inf.Type {
		case moderation.InflictionMuted:
			if inf.ExpiryDate != nil && *inf.ExpiryDate != 0 {
				inflictions.muteDuration.Store(*inf.ExpiryDate)
			}

			inflictions.muted.Store(true)
		case moderation.InflictionFrozen:
			inflictions.frozen.Store(true)
		}
	}

	// Apply side effects (e.g. SetImmobile) back on the world goroutine.
	handle.ExecWorld(func(_ *world.Tx, e world.Entity) {
		if p, ok := e.(*player.Player); ok {
			inflictions.handleActiveInflictions(p)
		}
	})
}

// Load queues a request to load the player's inflictions from the moderation service.
func (i *Inflictions) Load(handle *world.EntityHandle) {
	// Queue the request instead of processing it immediately
	select {
	case InflictionQueue <- inflictionRequest{
		handle:      handle,
		inflictions: i,
	}:
		// Successfully queued
	default:
		// Queue is full, log warning (can't access logger here, so just continue)
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

// inflictionLoadWorker processes infliction load requests with rate limiting.
//
// Uses a small semaphore (3) so we don't hammer the moderation API on
// startup or when many players join at once.
func inflictionLoadWorker() {
	defer inflictionWorkerWG.Done()

	semaphore := make(chan struct{}, 3)

	var activeRequestsWG sync.WaitGroup
	defer activeRequestsWG.Wait()

	for {
		select {
		case <-inflictionLoadWorkerShutdown:
			return
		case req, ok := <-inflictionLoadQueue:
			if !ok {
				return
			}

			select {
			case semaphore <- struct{}{}:
			case <-inflictionLoadWorkerShutdown:
				return
			}

			activeRequestsWG.Add(1)

			go func(handle *world.EntityHandle, inf *Inflictions) {
				defer activeRequestsWG.Done()
				defer func() { <-semaphore }()

				if handle == nil {
					return
				}

				inf.Load(handle)
			}(req.handle, req.inf)
		}
	}
}

// QueueLoad adds the player's handle to the infliction loading queue
func (i *Inflictions) QueueLoad(handle *world.EntityHandle) {
	// Avoid blocking the caller if the queue is full
	select {
	case inflictionLoadQueue <- inflictionLoadRequest{handle: handle, inf: i}:
		// Successfully queued
	default:
		// Queue is full, log this somewhere if needed
	}
}
