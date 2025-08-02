// Package session provides a session for the server.
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

var (
	// inflictionQueue is a buffered channel for loading player inflictions
	inflictionLoadQueue = make(chan inflictionLoadRequest, 50)
	// Used to signal worker shutdown
	inflictionLoadWorkerShutdown = make(chan struct{})
)

// inflictionLoadRequest represents a queued request to load player inflictions
type inflictionLoadRequest struct {
	handle *world.EntityHandle
	inf    *Inflictions
}

// init starts the background workers for processing infliction requests
func init() {
	go inflictionWorker()
	go inflictionLoadWorker()
}

// StopInflictionWorker stops all infliction workers gracefully
func StopInflictionWorker() {
	// Signal workers to shutdown
	close(inflictionWorkerShutdown)
	close(inflictionLoadWorkerShutdown)

	// Give workers time to finish active requests
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

					// Move ExecWorld outside of semaphore critical section
					// to prevent deadlock with condition variables
					go func() {
						// Add timeout to prevent infinite waiting
						done := make(chan struct{}, 1)
						go func() {
							defer close(done)
							handle.ExecWorld(func(_ *world.Tx, e world.Entity) {
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
						}()

						// Timeout after 30 seconds to prevent indefinite waiting
						select {
						case <-done:
							// Completed successfully
						case <-time.After(30 * time.Second):
							// Timeout - log warning but don't block
							// TODO: Add proper logging here
						}
					}()
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

// inflictionLoadWorker processes infliction load requests with rate limiting
func inflictionLoadWorker() {
	// Create a semaphore to limit concurrent API requests
	// Set a smaller number to reduce server load
	semaphore := make(chan struct{}, 3)

	for {
		select {
		case <-inflictionLoadWorkerShutdown:
			return
		case req, ok := <-inflictionLoadQueue:
			if !ok {
				return
			}

			// Acquire semaphore slot (blocks if max concurrent are already running)
			semaphore <- struct{}{}

			go func(handle *world.EntityHandle, inf *Inflictions) {
				defer func() {
					// Release semaphore slot when done
					<-semaphore
				}()

				// Skip if the entity doesn't exist anymore
				if handle == nil {
					return
				}

				// Use the original Load method directly
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
