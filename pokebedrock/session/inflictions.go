package session

import (
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
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
var InflictionQueue = make(chan inflictionRequest, 100)

// Used to signal worker shutdown
var inflictionWorkerShutdown = make(chan struct{})

// inflictionRequest represents a queued request to load player inflictions
type inflictionRequest struct {
	handle      *world.EntityHandle
	inflictions *Inflictions
}

// init starts the background worker for processing infliction requests
func init() {
	go inflictionWorker()
}

// StopInflictionWorker stops the infliction worker goroutine gracefully
func StopInflictionWorker() {
	// Signal worker to shutdown
	close(inflictionWorkerShutdown)

	// Give workers time to finish active requests (up to 3 seconds)
	timeout := time.NewTimer(3 * time.Second)
	<-timeout.C
}

// inflictionWorker processes queued infliction load requests with rate limiting
func inflictionWorker() {
	// Create a semaphore using a buffered channel to limit concurrent requests
	semaphore := make(chan struct{}, maxConcurrentInflictionRequests)

	// Track active requests to ensure we can shut down cleanly
	activeRequests := make(chan struct{}, maxConcurrentInflictionRequests)

	for {
		select {
		case <-inflictionWorkerShutdown:
			// Wait for all active requests to finish before exiting
			for i := 0; i < len(activeRequests); i++ {
				<-activeRequests
			}
			return
		case req, ok := <-InflictionQueue:
			if !ok {
				// Channel closed, exit worker
				return
			}

			// Acquire semaphore slot (blocks if max concurrent requests are already running)
			select {
			case semaphore <- struct{}{}:
				// Track active request
				activeRequests <- struct{}{}

				// Process request in a goroutine
				go func(handle *world.EntityHandle, inflictions *Inflictions) {
					defer func() {
						// Release semaphore slot when done
						<-semaphore
						// Mark request as complete
						<-activeRequests
					}()

					handle.ExecWorld(func(tx *world.Tx, e world.Entity) {
						p, ok := e.(*player.Player)
						if !ok {
							return
						}

						modSvc := moderation.GlobalService()
						if modSvc == nil {
							return
						}

						resp, err := modSvc.InflictionOfPlayer(p)
						if err != nil {
							return
						}

						for _, inf := range resp.CurrentInflictions {
							switch inf.Type {
							case moderation.InflictionMuted:
								expiry := inf.ExpiryDate
								if expiry != nil && *expiry != 0 {
									inflictions.muteDuration.Store(*expiry)
								}
								inflictions.muted.Store(true)
							case moderation.InflictionFrozen:
								inflictions.frozen.Store(true)
							}
						}

						inflictions.handleActiveInflictions(p)
					})
				}(req.handle, req.inflictions)
			case <-inflictionWorkerShutdown:
				// Worker is shutting down, don't start new requests
				return
			}
		}
	}
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
