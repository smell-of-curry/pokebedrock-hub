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

	poke := &PokeBedrock{
		log:  log,
		conf: conf,

		c:          make(chan struct{}),
		resManager: resManager,
	}
	// TODO: Enable when these get fixed.
	// poke.loadTranslations(&c)
	if err = poke.loadLocales(); err != nil {
		return nil, err
	}
	poke.loadCommands()

	c.ReadOnlyWorld = true
	c.Generator = func(dim world.Dimension) world.Generator { // ensures that no new chunks are generated.
		return world.NopGenerator{}
	}
	c.StatusProvider = status.NewProvider(c.Name, c.Name) // ensures synchronized server count display.
	c.Allower = &Allower{}

	poke.srv = c.New()
	poke.srv.CloseOnProgramEnd()

	poke.loadServices()

	return poke, nil
}

// Start ...
func (poke *PokeBedrock) Start() {
	poke.srv.Listen()
	poke.handleWorld()

	for pl := range poke.srv.Accept() {
		poke.accept(pl)
	}

	close(poke.c)
}

// handleWorld ...
func (poke *PokeBedrock) handleWorld() {
	w := poke.World()

	w.StopWeatherCycle()
	w.StopRaining()
	w.StopThundering()
	w.SetDefaultGameMode(world.GameModeAdventure)
	w.SetTime(6000)
	w.StopTime()
	w.SetTickRange(0)

	poke.loadServers()
	poke.loadSlappers()
	go poke.startTicking()
}

// loadTranslations ...
func (poke *PokeBedrock) loadTranslations(c *server.Config) {
	conf := poke.conf
	c.JoinMessage = translation.MessageJoin(conf.Translation.MessageJoin)
	c.QuitMessage = translation.MessageQuit(conf.Translation.MessageLeave)
	c.ShutdownMessage = translation.MessageServerDisconnect(conf.Translation.MessageServerDisconnect)
}

// loadLocales ...
func (poke *PokeBedrock) loadLocales() error {
	path := poke.conf.PokeBedrock.LocalePath
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
func (poke *PokeBedrock) loadCommands() {
	cmd.Register(command.NewModerate(rank.Moderator))
}

// loadServices ...
func (poke *PokeBedrock) loadServices() {
	rank.NewService(poke.log, poke.conf.Service.RolesURL)
	moderation.NewService(poke.log, poke.conf.Service.ModerationURL, poke.conf.Service.ModerationKey)
}

// loadServers ...
func (poke *PokeBedrock) loadServers() {
	cfgs, err := srv.ReadAll(poke.conf.PokeBedrock.ServerPath)
	if err != nil {
		panic(err)
	}

	for _, cfg := range cfgs {
		srv.Register(
			srv.NewServer(poke.log, cfg),
		)
	}

	srv.UpdateAll()
}

// loadSlappers ...
func (poke *PokeBedrock) loadSlappers() {
	w := poke.World()
	cfgs, err := slapper.ReadAll(poke.conf.PokeBedrock.SlapperPath)
	if err != nil {
		panic(err)
	}

	<-w.Exec(func(tx *world.Tx) {
		slapper.SummonAll(poke.log, cfgs, tx, poke.resManager)
	})
}

// startTicking ...
func (poke *PokeBedrock) startTicking() {
	w := poke.World()
	t := time.NewTicker(time.Second * 1)
	defer t.Stop()

	var counter int
	f := func(n int) bool {
		return counter%n == 0
	}

	for {
		select {
		case <-poke.c:
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
func (poke *PokeBedrock) accept(p *player.Player) {
	// Create and set the player handler.
	h := handler.NewPlayerHandler(p)
	p.Handle(h)

	h.HandleJoin(p, poke.World())
}

// World ...
func (poke *PokeBedrock) World() *world.World {
	return poke.srv.World()
}
