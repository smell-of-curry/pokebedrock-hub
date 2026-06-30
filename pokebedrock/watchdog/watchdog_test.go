package watchdog

import (
	"testing"
	"time"
)

// newTestWatchdog builds a Watchdog with defaults applied but no world, which
// is enough to exercise the pure evaluate/shouldAlert logic.
func newTestWatchdog() *Watchdog {
	return &Watchdog{
		conf:      Config{}.withDefaults(),
		lastAlert: make(map[condition]time.Time),
	}
}

func hasCond(alerts []alert, c condition) bool {
	for _, a := range alerts {
		if a.cond == c {
			return true
		}
	}
	return false
}

func TestEvaluate(t *testing.T) {
	w := newTestWatchdog()

	cases := []struct {
		name string
		s    snapshot
		want []condition
	}{
		{"healthy", snapshot{goroutines: 200, heapAlloc: 500 << 20}, nil},
		{"world stall", snapshot{goroutines: 200, worldStalled: true}, []condition{condWorldStall}},
		{"goroutine pileup", snapshot{goroutines: defaultGoroutineThreshold}, []condition{condGoroutines}},
		{"heap pressure", snapshot{goroutines: 10, heapAlloc: defaultHeapAllocThreshold}, []condition{condHeap}},
		{
			"deadlock signature: stall + pileup",
			snapshot{goroutines: 4114, heapAlloc: 17 << 30, worldStalled: true},
			[]condition{condWorldStall, condGoroutines, condHeap},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := w.evaluate(tc.s)
			if len(got) != len(tc.want) {
				t.Fatalf("evaluate(%+v) = %d alerts, want %d (%v)", tc.s, len(got), len(tc.want), got)
			}
			for _, c := range tc.want {
				if !hasCond(got, c) {
					t.Errorf("evaluate(%+v) missing condition %q", tc.s, c)
				}
			}
		})
	}

	// Stall must be reported first so the most actionable alert leads.
	if got := w.evaluate(snapshot{goroutines: 5000, worldStalled: true}); got[0].cond != condWorldStall {
		t.Errorf("expected world stall first, got %q", got[0].cond)
	}
}

func TestShouldAlertCooldown(t *testing.T) {
	w := newTestWatchdog()
	w.conf.AlertCooldown = time.Minute
	start := time.Unix(1_000, 0)

	if !w.shouldAlert(condWorldStall, start) {
		t.Fatal("first alert should fire")
	}
	if w.shouldAlert(condWorldStall, start.Add(30*time.Second)) {
		t.Error("alert within cooldown should be suppressed")
	}
	if !w.shouldAlert(condWorldStall, start.Add(2*time.Minute)) {
		t.Error("alert after cooldown should fire")
	}
	// Different conditions are throttled independently.
	if !w.shouldAlert(condGoroutines, start.Add(30*time.Second)) {
		t.Error("distinct condition should not share cooldown")
	}
}

func TestHeapProbeDisabled(t *testing.T) {
	w := newTestWatchdog()
	w.conf.HeapAllocThreshold = 0 // disabled

	if got := w.evaluate(snapshot{goroutines: 10, heapAlloc: 100 << 30}); len(got) != 0 {
		t.Errorf("heap probe disabled, want no alerts, got %v", got)
	}
}
