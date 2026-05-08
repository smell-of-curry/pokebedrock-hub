package session

import (
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/df-mc/atomic"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/internal"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
)

// rankWorkerCount is the number of background workers consuming
// rankUpdateCh. Roles are fetched via blocking HTTP, so multiple workers
// give us parallelism without needing the legacy double-queue + semaphore
// scheme.
const rankWorkerCount = 3

var (
	// rankUpdateCh is the single queue of pending rank updates. Workers
	// consume from this channel and perform the HTTP call off the world's
	// transaction goroutine; results are applied via ExecWorld.
	rankUpdateCh = make(chan rankUpdate, internal.DefaultChannelBufferSize)

	// rankShutdown signals all rank workers to exit.
	rankShutdown = make(chan struct{})

	// rankWorkerWG is signalled when every rank worker has exited so
	// shutdown callers can wait deterministically.
	rankWorkerWG sync.WaitGroup

	// rankShutdownOnce guards rankShutdown so multiple shutdown callers
	// don't double-close.
	rankShutdownOnce sync.Once
)

// rankUpdate is the unit of work consumed by rank workers.
type rankUpdate struct {
	xuid   string
	handle *world.EntityHandle
	ranks  *Ranks

	// notify controls whether the worker sends a "rank.fetching" popup
	// before the HTTP call. Used so the original NewPlayerHandler path
	// can stay quiet while sync-rank actions inform the user.
	notify bool
}

func init() {
	for range rankWorkerCount {
		rankWorkerWG.Add(1)
		go rankWorker()
	}
}

// StopRankChannel signals every rank worker to exit and waits up to a few
// seconds for them to drain. Safe to call multiple times.
func StopRankChannel() {
	rankShutdownOnce.Do(func() {
		close(rankShutdown)
	})

	done := make(chan struct{})
	go func() {
		rankWorkerWG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}

// StopRankLoadWorker is retained for backwards-compatible call sites; the
// secondary load worker has been removed in favour of a single queue.
func StopRankLoadWorker() {
	StopRankChannel()
}

// rankWorker consumes rankUpdate values from rankUpdateCh and processes
// them one at a time. The HTTP call is performed on this goroutine, NOT on
// the world transaction goroutine, per the no-blocking-io-in-execworld
// rule.
func rankWorker() {
	defer rankWorkerWG.Done()

	for {
		select {
		case <-rankShutdown:
			return
		case update, ok := <-rankUpdateCh:
			if !ok {
				return
			}

			processRankUpdate(update)
		}
	}
}

// processRankUpdate runs a single rank update end-to-end.
func processRankUpdate(update rankUpdate) {
	if update.handle == nil || update.ranks == nil {
		return
	}

	if update.notify {
		update.handle.ExecWorld(func(_ *world.Tx, e world.Entity) {
			if p, ok := e.(*player.Player); ok {
				p.SendJukeboxPopup(locale.Translate("rank.fetching"))
			}
		})
	}

	roles, err := rank.GlobalService().RolesOfXUID(update.xuid)
	if err != nil {
		update.ranks.SetRanks([]rank.Rank{rank.UnLinked})

		msg := text.Colourf("<red>%s</red>", rank.RolesError(err))
		update.handle.ExecWorld(func(_ *world.Tx, e world.Entity) {
			if p, ok := e.(*player.Player); ok {
				p.SendJukeboxPopup(msg)
				p.Message(msg)
			}
		})

		return
	}

	ranks := rank.RolesToRanks(roles)
	if len(ranks) == 0 {
		// Player has no valid mapped roles; default to Trainer.
		ranks = []rank.Rank{rank.Trainer}
	}

	update.ranks.SetRanks(ranks)
	update.ranks.SetLastRankFetch(time.Now())

	highest := update.ranks.HighestRank()
	syncedMsg := text.Colourf("<green>%s</green>", locale.Translate("rank.synced", highest.Name()))

	update.handle.ExecWorld(func(_ *world.Tx, e world.Entity) {
		p, ok := e.(*player.Player)
		if !ok {
			return
		}

		p.SendJukeboxPopup(syncedMsg)
		p.Message(syncedMsg)
		p.SetNameTag(highest.NameTag(p.Name()))
	})
}

// Ranks tracks a player's resolved ranks and the last time they were
// fetched.
type Ranks struct {
	rankMu sync.Mutex
	ranks  []rank.Rank

	lastRankFetch atomic.Value[time.Time]
}

// NewRanks creates an empty Ranks container.
func NewRanks() *Ranks {
	r := &Ranks{
		ranks: make([]rank.Rank, 0),
	}
	r.lastRankFetch.Store(time.Time{})

	return r
}

// Load enqueues a rank fetch for the given player. The call returns as
// soon as the request is accepted by the worker pool (or dropped, if the
// queue is full); the player will be updated asynchronously.
func (r *Ranks) Load(xuid string, handle *world.EntityHandle) {
	r.enqueue(rankUpdate{xuid: xuid, handle: handle, ranks: r, notify: true})
}

// QueueLoad enqueues a rank fetch the same as Load. It is retained as a
// distinct method so existing call sites continue to compile.
func (r *Ranks) QueueLoad(xuid string, handle *world.EntityHandle) {
	r.enqueue(rankUpdate{xuid: xuid, handle: handle, ranks: r, notify: true})
}

func (r *Ranks) enqueue(update rankUpdate) {
	select {
	case rankUpdateCh <- update:
		return
	case <-rankShutdown:
		return
	default:
	}

	if update.handle == nil {
		return
	}

	update.handle.ExecWorld(func(_ *world.Tx, e world.Entity) {
		if p, ok := e.(*player.Player); ok {
			p.SendJukeboxPopup(locale.Translate("rank.update.queue.full"))
		}
	})
}

// SetRanks updates the player's ranks atomically and re-sorts them so that
// the highest rank is always last.
func (r *Ranks) SetRanks(ranks []rank.Rank) {
	r.rankMu.Lock()
	defer r.rankMu.Unlock()

	r.ranks = ranks
	sort.SliceStable(r.ranks, func(i, j int) bool {
		return r.ranks[i] < r.ranks[j]
	})
}

// HighestRank returns the player's highest rank, or UnLinked if none are
// set.
func (r *Ranks) HighestRank() rank.Rank {
	r.rankMu.Lock()
	defer r.rankMu.Unlock()

	if len(r.ranks) == 0 {
		return rank.UnLinked
	}

	return r.ranks[len(r.ranks)-1]
}

// Ranks returns a copy of the player's ranks.
func (r *Ranks) Ranks() []rank.Rank {
	r.rankMu.Lock()
	defer r.rankMu.Unlock()

	return slices.Clone(r.ranks)
}

// HasRank reports whether the player has the given rank.
func (r *Ranks) HasRank(ra rank.Rank) bool {
	r.rankMu.Lock()
	defer r.rankMu.Unlock()

	return slices.Contains(r.ranks, ra)
}

// HasRankOrHigher reports whether the player has the given rank or any
// rank above it.
func (r *Ranks) HasRankOrHigher(ra rank.Rank) bool {
	return r.HighestRank() >= ra
}

// LastRankFetch returns the last time ranks were fetched successfully.
func (r *Ranks) LastRankFetch() time.Time {
	return r.lastRankFetch.Load()
}

// SetLastRankFetch records the last time ranks were fetched successfully.
func (r *Ranks) SetLastRankFetch(t time.Time) {
	r.lastRankFetch.Store(t)
}
