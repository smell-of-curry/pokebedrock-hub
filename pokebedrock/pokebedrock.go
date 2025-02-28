package pokebedrock

import (
	"log/slog"
	"time"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/handler"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/queue"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/resources"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/slapper"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/status"
)

// PokeBedrock ...
type PokeBedrock struct {
	log  *slog.Logger
	conf Config

	srv *server.Server
	w   *world.World

	c chan struct{}
}

// New ...
func New(log *slog.Logger, conf Config) *PokeBedrock {
	// Initialize resource pack manager and check for updates
	resManager := resources.NewManager(conf.UserConfig.Resources.Folder)
	if err := resManager.CheckAndUpdate(); err != nil {
		log.Error("Failed to check/update resource pack", "error", err)
	}

	c, err := conf.UserConfig.Config(log)
	if err != nil {
		panic(err)
	}

	p := &PokeBedrock{
		log:  log,
		conf: conf,

		c: make(chan struct{}),
	}

	c.ReadOnlyWorld = true
	c.Generator = func(dim world.Dimension) world.Generator { // ensures that no new chunks are generated.
		return world.NopGenerator{}
	}
	c.StatusProvider = status.NewProvider(c.Name, c.Name) // ensures synchronized server count display.

	p.srv = c.New()
	p.w = p.srv.World()
	p.srv.CloseOnProgramEnd()
	return p
}

// Start ...
func (p *PokeBedrock) Start() error {
	p.srv.Listen()
	p.handleWorld()

	for pl := range p.srv.Accept() {
		p.accept(pl)
	}

	close(p.c)
	return nil
}

// handleWorld ...
func (p *PokeBedrock) handleWorld() {
	w := p.w
	w.Handle(handler.WorldHandler{})

	w.StopWeatherCycle()
	w.StopRaining()
	w.StopThundering()
	w.SetDefaultGameMode(world.GameModeAdventure)
	w.SetTime(6000)
	w.StopTime()
	w.SetTickRange(0)

	l := world.NewLoader(32, w, world.NopViewer{})
	<-w.Exec(func(tx *world.Tx) {
		l.Move(tx, w.Spawn().Vec3Middle())
		l.Load(tx, 250)
	})

	p.loadServers()
	p.loadSlappers()
	go p.startTicking()
}

// loadServers ...
func (p *PokeBedrock) loadServers() {
	cfgs, err := srv.ReadAll(p.conf.PokeBedrock.ServerPath)
	if err != nil {
		panic(err)
	}

	for _, cfg := range cfgs {
		srv.Register(
			srv.New(p.log, &cfg),
		)
	}

	srv.UpdateAll()
}

// loadSlappers ...
func (p *PokeBedrock) loadSlappers() {
	w := p.w
	cfgs, err := slapper.ReadAll(p.conf.PokeBedrock.SlapperPath)
	if err != nil {
		panic(err)
	}

	<-w.Exec(func(tx *world.Tx) {
		slapper.SummonAll(p.log, cfgs, tx)
	})
}

// startTicking ...
func (p *PokeBedrock) startTicking() {
	w := p.w
	t := time.NewTicker(time.Second * 1)
	defer t.Stop()

	var counter int
	f := func(n int) bool {
		return counter%n == 0
	}

	for {
		select {
		case <-p.c:
			return
		case <-t.C:
			w.Exec(func(tx *world.Tx) {
				counter++

				queue.QueueManager.Update(tx)

				switch true {
				case f(10):
					srv.UpdateAll()
				case f(5):
					slapper.UpdateAll(tx)
				}
			})
		}
	}
}

// accept ...
func (p *PokeBedrock) accept(pl *player.Player) {
	h := handler.NewPlayerHandler()
	pl.Handle(h)

	h.HandleJoin(pl, p.w)
}
