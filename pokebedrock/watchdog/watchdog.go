// Package watchdog provides a lightweight runtime health monitor for the hub.
//
// It periodically probes the dragonfly world tick and process-level metrics
// (goroutine count, heap usage) and raises alerts (structured error log +
// Sentry) when a threshold is breached or the world stops ticking.
//
// The world-stall probe exists specifically to catch the failure mode where
// the single world-tick goroutine wedges (e.g. a re-entrant World.Exec
// deadlock): when that happens a probe transaction never completes, so instead
// of silently dropping every login the watchdog fires a fatal alert with a full
// goroutine dump attached.
package watchdog

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/getsentry/sentry-go"
)

const (
	defaultCheckInterval      = 30 * time.Second
	defaultWorldExecTimeout   = 10 * time.Second
	defaultGoroutineThreshold = 800
	defaultHeapAllocThreshold = 2 << 30 // 2 GiB
	defaultAlertCooldown      = 5 * time.Minute

	// goroutineDumpLimit bounds the size of the goroutine dump attached to
	// alerts so a pathological pile-up can't produce a multi-MB Sentry event.
	goroutineDumpLimit = 1 << 20 // 1 MiB
)

// condition identifies a distinct unhealthy state the watchdog can detect.
type condition string

const (
	condWorldStall condition = "world_stall"
	condGoroutines condition = "goroutine_pileup"
	condHeap       condition = "heap_pressure"
)

// Config tunes the watchdog. Zero-valued fields are replaced with sensible
// defaults by withDefaults, so a zero Config is valid.
type Config struct {
	// CheckInterval is how often the watchdog runs its probes.
	CheckInterval time.Duration
	// WorldExecTimeout is how long a probe world transaction may take before
	// the world is considered stalled.
	WorldExecTimeout time.Duration
	// GoroutineThreshold is the goroutine count at or above which an alert is
	// raised. A healthy hub sits in the low hundreds; a login pile-up climbs
	// into the thousands.
	GoroutineThreshold int
	// HeapAllocThreshold is the heap-allocated byte count at or above which an
	// alert is raised. A value of 0 disables the heap probe.
	HeapAllocThreshold uint64
	// AlertCooldown is the minimum time between repeat Sentry alerts for the
	// same condition. Every breach is still logged; this only throttles Sentry.
	AlertCooldown time.Duration
}

// withDefaults returns a copy of c with any zero-valued field replaced by its
// default.
//
// @returns a Config safe to use directly.
func (c Config) withDefaults() Config {
	if c.CheckInterval <= 0 {
		c.CheckInterval = defaultCheckInterval
	}
	if c.WorldExecTimeout <= 0 {
		c.WorldExecTimeout = defaultWorldExecTimeout
	}
	if c.GoroutineThreshold <= 0 {
		c.GoroutineThreshold = defaultGoroutineThreshold
	}
	if c.HeapAllocThreshold == 0 {
		c.HeapAllocThreshold = defaultHeapAllocThreshold
	}
	if c.AlertCooldown <= 0 {
		c.AlertCooldown = defaultAlertCooldown
	}
	return c
}

// Watchdog monitors hub health on a background goroutine.
type Watchdog struct {
	log   *slog.Logger
	world *world.World
	conf  Config

	mu        sync.Mutex
	lastAlert map[condition]time.Time

	stop     chan struct{}
	stopOnce sync.Once
}

// New creates a Watchdog that probes the given world. Call Start to begin
// monitoring and Stop to shut it down.
//
// @param log The logger used for alert and lifecycle messages.
// @param w The world whose tick loop is probed for stalls.
// @param conf Tuning parameters; zero-valued fields use defaults.
// @returns the configured Watchdog.
func New(log *slog.Logger, w *world.World, conf Config) *Watchdog {
	return &Watchdog{
		log:       log,
		world:     w,
		conf:      conf.withDefaults(),
		lastAlert: make(map[condition]time.Time),
		stop:      make(chan struct{}),
	}
}

// Start launches the monitoring goroutine. It returns immediately.
func (w *Watchdog) Start() {
	w.log.Info("watchdog started",
		"check_interval", w.conf.CheckInterval,
		"world_exec_timeout", w.conf.WorldExecTimeout,
		"goroutine_threshold", w.conf.GoroutineThreshold,
		"heap_alloc_threshold_bytes", w.conf.HeapAllocThreshold,
	)
	go w.loop()
}

// Stop signals the monitoring goroutine to exit. Safe to call more than once.
func (w *Watchdog) Stop() {
	w.stopOnce.Do(func() { close(w.stop) })
}

func (w *Watchdog) loop() {
	t := time.NewTicker(w.conf.CheckInterval)
	defer t.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-t.C:
			w.runCheck()
		}
	}
}

// snapshot is the set of health signals sampled in a single check. Separating
// sampling from evaluation keeps evaluate pure and unit-testable without a
// running world or runtime.
type snapshot struct {
	goroutines   int
	heapAlloc    uint64
	worldStalled bool
}

// alert is a single breached condition ready to be dispatched.
type alert struct {
	cond    condition
	level   sentry.Level
	message string
}

// runCheck samples current health signals and dispatches any resulting alerts.
func (w *Watchdog) runCheck() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	s := snapshot{
		goroutines:   runtime.NumGoroutine(),
		heapAlloc:    ms.HeapAlloc,
		worldStalled: w.worldStalled(),
	}

	for _, a := range w.evaluate(s) {
		w.dispatch(a, s)
	}
}

// evaluate returns the alerts implied by a snapshot. It is pure: it does not
// touch process state, time, or cooldowns, which makes it directly testable.
//
// @param s The sampled health signals.
// @returns the breached conditions, in severity order (stall first).
func (w *Watchdog) evaluate(s snapshot) []alert {
	var out []alert

	if s.worldStalled {
		out = append(out, alert{
			cond:  condWorldStall,
			level: sentry.LevelFatal,
			message: fmt.Sprintf(
				"world tick stalled: probe transaction did not complete within %s "+
					"(world goroutine wedged, likely a re-entrant World.Exec deadlock); "+
					"new players cannot join",
				w.conf.WorldExecTimeout),
		})
	}
	if s.goroutines >= w.conf.GoroutineThreshold {
		out = append(out, alert{
			cond:    condGoroutines,
			level:   sentry.LevelError,
			message: fmt.Sprintf("goroutine count high: %d (threshold %d)", s.goroutines, w.conf.GoroutineThreshold),
		})
	}
	if w.conf.HeapAllocThreshold > 0 && s.heapAlloc >= w.conf.HeapAllocThreshold {
		out = append(out, alert{
			cond:    condHeap,
			level:   sentry.LevelError,
			message: fmt.Sprintf("heap usage high: %d bytes (threshold %d)", s.heapAlloc, w.conf.HeapAllocThreshold),
		})
	}

	return out
}

// worldStalled submits a no-op probe transaction and reports whether it failed
// to complete within WorldExecTimeout.
//
// @returns true if the world tick goroutine did not process the probe in time.
func (w *Watchdog) worldStalled() bool {
	done := w.world.Exec(func(*world.Tx) {})
	select {
	case <-done:
		return false
	case <-time.After(w.conf.WorldExecTimeout):
		// ponytail: a genuinely stalled world leaks this one probe transaction
		// in the world queue until (if) it recovers. Bounded to one per
		// CheckInterval, and a stalled process is already doomed to a restart,
		// so we don't attempt to reap it.
		return true
	}
}

// dispatch always logs the alert and, subject to the per-condition cooldown,
// reports it to Sentry with diagnostic context attached.
func (w *Watchdog) dispatch(a alert, s snapshot) {
	w.log.Error("watchdog alert",
		"condition", string(a.cond),
		"detail", a.message,
		"goroutines", s.goroutines,
		"heap_alloc_bytes", s.heapAlloc,
	)

	if !w.shouldAlert(a.cond, time.Now()) {
		return
	}

	includeDump := a.cond == condWorldStall || s.goroutines >= w.conf.GoroutineThreshold
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(a.level)
		scope.SetTag("watchdog_condition", string(a.cond))

		ctx := sentry.Context{
			"goroutines":       s.goroutines,
			"heap_alloc_bytes": s.heapAlloc,
			"world_stalled":    s.worldStalled,
		}
		if includeDump {
			ctx["goroutine_dump"] = goroutineDump()
		}
		scope.SetContext("watchdog", ctx)

		sentry.CaptureMessage("watchdog: " + a.message)
	})
	// Flush eagerly: a world stall often precedes the process being killed and
	// restarted, and we don't want to lose the event.
	sentry.Flush(2 * time.Second)
}

// shouldAlert reports whether a Sentry alert for the given condition is allowed
// right now, recording the time when it returns true so subsequent calls are
// throttled by AlertCooldown.
//
// @param c The condition being alerted on.
// @param now The current time (injected for testability).
// @returns true if a Sentry alert should be sent.
func (w *Watchdog) shouldAlert(c condition, now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if last, ok := w.lastAlert[c]; ok && now.Sub(last) < w.conf.AlertCooldown {
		return false
	}
	w.lastAlert[c] = now
	return true
}

// goroutineDump returns the stacks of all running goroutines, capped at
// goroutineDumpLimit bytes.
//
// @returns the formatted goroutine stack dump.
func goroutineDump() string {
	buf := make([]byte, goroutineDumpLimit)
	n := runtime.Stack(buf, true)
	return string(buf[:n])
}
