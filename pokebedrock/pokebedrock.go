package pokebedrock

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/gin-gonic/gin"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/command"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/handler"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/identity"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/moderation"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/queue"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/resources"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/session"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/slapper"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/status"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/translation"
	"golang.org/x/text/language"
)

// PokeBedrock represents the main server struct.
// It holds configuration, logging, and manages various server components.
type PokeBedrock struct {
	log  *slog.Logger
	conf Config

	srv        *server.Server
	resManager *resources.Manager

	c chan struct{}
}

// NewPokeBedrock creates a new instance of PokeBedrock.
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
	poke.setupGin()

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

// Start begins the server's main loop, accepting connections and handling players.
// It blocks until the server is closed.
func (poke *PokeBedrock) Start() {
	poke.srv.Listen()
	poke.handleWorld()

	for pl := range poke.srv.Accept() {
		poke.accept(pl)
	}

	poke.Close()
}

// handleWorld initializes and configures the world settings.
// It sets up the environment and starts background processes.
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

// setupGin sets up gin for the gobds proxy.
func (poke *PokeBedrock) setupGin() {
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	router.Use(func(c *gin.Context) {
		if c.GetHeader("authorization") != poke.conf.Service.IdentityKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	})
	router.GET("/identity/:name", func(c *gin.Context) {
		name := c.Param("name")
		req, exists := identity.GlobalFactory().Of(name)
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"reason": "no player found"})
			return
		}

		if time.Now().After(req.Expiration) {
			identity.GlobalFactory().Remove(name)

			c.JSON(http.StatusNotFound, gin.H{"reason": "request expired"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"xuid": req.XUID,
		})
	})
}

// loadTranslations loads all the translation used in dragonfly.
func (poke *PokeBedrock) loadTranslations(c *server.Config) {
	conf := poke.conf
	c.JoinMessage = translation.MessageJoin(conf.Translation.MessageJoin)
	c.QuitMessage = translation.MessageQuit(conf.Translation.MessageLeave)
	c.ShutdownMessage = translation.MessageServerDisconnect(conf.Translation.MessageServerDisconnect)
}

// loadLocales registers all the locales active on the server.
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

// loadCommands registers all the commands on the server.
func (poke *PokeBedrock) loadCommands() {
	cmd.Register(command.NewModerate(rank.Moderator))
	cmd.Register(command.NewKick(rank.Moderator))
	cmd.Register(command.NewList(rank.Trainer))
}

// loadServices loads all the services.
func (poke *PokeBedrock) loadServices() {
	rank.NewService(poke.log, poke.conf.Service.RolesURL)
	moderation.NewService(poke.log, poke.conf.Service.ModerationURL, poke.conf.Service.ModerationKey)
}

// loadServers loads all the server configurations from the specified path
// and registers them with the server manager. It panics if server configurations
// cannot be read.
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

// loadSlappers loads and spawns all NPC entities (slappers) from the configuration.
// It reads slapper configurations from the specified path and summons them in the world.
// Panics if slapper configurations cannot be read.
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

// startTicking begins the periodic ticking process for the server.
// It executes server updates, manages queues, and periodically triggers specific actions.
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

	// Send details of the player to the moderation service
	moderation.GlobalService().SendDetailsOf(p)

	h.HandleJoin(p, poke.World())
}

// Close closes the server and all its associated services.
func (poke *PokeBedrock) Close() {
	poke.log.Debug("Closing Moderation Service...")
	moderation.GlobalService().Stop()
	poke.log.Debug("Closing Rank Service...")
	rank.GlobalService().Stop()
	poke.log.Debug("Stopping Rank Channel...")
	session.StopRankChannel()
	poke.log.Debug("Stopping Rank Load Worker...")
	session.StopRankLoadWorker()
	poke.log.Debug("Stopping Infliction Worker...")
	session.StopInflictionWorker()
	poke.log.Debug("Stopping Queue Manager...")
	close(poke.c)
}

// World returns the default world.
func (poke *PokeBedrock) World() *world.World {
	return poke.srv.World()
}
