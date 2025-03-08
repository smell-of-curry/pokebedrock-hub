package pokebedrock

import (
	"log/slog"
	"time"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/command"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/handler"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/moderation"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/queue"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/resources"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/slapper"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/status"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/translation"
	"golang.org/x/text/language"
)

// PokeBedrock ...
type PokeBedrock struct {
	log  *slog.Logger
	conf Config

	srv        *server.Server
	resManager *resources.Manager

	c chan struct{}
}

// NewPokeBedrock ...
func NewPokeBedrock(log *slog.Logger, conf Config) (*PokeBedrock, error) {
	// Initialize resource pack manager and check for updates.
	resManager := resources.NewManager(log, conf.UserConfig.Resources.Folder)
	if err := resManager.CheckAndUpdate(); err != nil {
		log.Error("failed to check/update resource pack", "error", err)
	}

	log.Info("Starting Server...")

	c, err := conf.UserConfig.Config(log)
	if err != nil {
		return nil, err
	}

	p := &PokeBedrock{
		log:  log,
		conf: conf,

		c:          make(chan struct{}),
		resManager: resManager,
	}
	// TODO: Enable when these get fixed.
	// p.loadTranslations(&c)
	if err = p.loadLocales(); err != nil {
		return nil, err
	}
	p.loadCommands()

	c.ReadOnlyWorld = true
	c.Generator = func(dim world.Dimension) world.Generator { // ensures that no new chunks are generated.
		return world.NopGenerator{}
	}
	c.StatusProvider = status.NewProvider(c.Name, c.Name) // ensures synchronized server count display.
	c.Allower = &Allower{}

	p.srv = c.New()
	p.srv.CloseOnProgramEnd()

	p.loadServices()

	return p, nil
}

// Start ...
func (p *PokeBedrock) Start() {
	p.srv.Listen()
	p.handleWorld()

	for pl := range p.srv.Accept() {
		p.accept(pl)
	}

	close(p.c)
}

// handleWorld ...
func (p *PokeBedrock) handleWorld() {
	w := p.World()
	w.Handle(handler.WorldHandler{})

	w.StopWeatherCycle()
	w.StopRaining()
	w.StopThundering()
	w.SetDefaultGameMode(world.GameModeAdventure)
	w.SetTime(6000)
	w.StopTime()
	w.SetTickRange(0)

	p.loadServers()
	p.loadSlappers()
	go p.startTicking()
}

// loadTranslations ...
func (p *PokeBedrock) loadTranslations(c *server.Config) {
	conf := p.conf
	c.JoinMessage = translation.MessageJoin(conf.Translation.MessageJoin)
	c.QuitMessage = translation.MessageQuit(conf.Translation.MessageLeave)
	c.ShutdownMessage = translation.MessageServerDisconnect(conf.Translation.MessageServerDisconnect)
}

// loadLocales ...
func (p *PokeBedrock) loadLocales() error {
	path := p.conf.PokeBedrock.LocalePath
	locales := []language.Tag{
		language.English,
	}
	for _, l := range locales {
		if err := locale.Register(l, path); err != nil {
			return err
		}
	}
	return nil
}

// loadCommands ...
func (p *PokeBedrock) loadCommands() {
	cmd.Register(command.NewModerate(rank.Moderator))
}

// loadServices ...
func (p *PokeBedrock) loadServices() {
	rank.NewService(p.log, p.conf.Service.RolesURL)
	moderation.NewService(p.log, p.conf.Service.ModerationURL, p.conf.Service.ModerationKey)
}

// loadServers ...
func (p *PokeBedrock) loadServers() {
	cfgs, err := srv.ReadAll(p.conf.PokeBedrock.ServerPath)
	if err != nil {
		panic(err)
	}

	for _, cfg := range cfgs {
		srv.Register(
			srv.NewServer(p.log, cfg),
		)
	}

	srv.UpdateAll()
}

// loadSlappers ...
func (p *PokeBedrock) loadSlappers() {
	w := p.World()
	cfgs, err := slapper.ReadAll(p.conf.PokeBedrock.SlapperPath)
	if err != nil {
		panic(err)
	}

	<-w.Exec(func(tx *world.Tx) {
		slapper.SummonAll(p.log, cfgs, tx, p.resManager)
	})
}

// startTicking ...
func (p *PokeBedrock) startTicking() {
	w := p.World()
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

// accept handles a new player joining the server.
func (p *PokeBedrock) accept(pl *player.Player) {
	// Create and set the player handler.
	h := handler.NewPlayerHandler(pl)
	pl.Handle(h)

	h.HandleJoin(pl, p.World())
}

// World ...
func (p *PokeBedrock) World() *world.World {
	return p.srv.World()
}
