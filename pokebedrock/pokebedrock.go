// Package pokebedrock provides a Minecraft Bedrock Edition server hub implementation.
// It manages player queues, server navigation, NPC entities, authentication,
// moderation, and rank systems for the PokeBedrock network.
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
	"github.com/sandertv/gophertunnel/minecraft/text"
	"golang.org/x/text/language"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/authentication"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/command"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/handler"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/locale"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/moderation"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/queue"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/rank"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/resources"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/restart"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/session"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/slapper"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/status"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/vpn"
)

const (
	// defaultWorldTime is the default time to set in the world (6000 = noon)
	defaultWorldTime = 6000

	// worldLoadRadius is the maximum radius to load around spawn
	worldLoadRadius = 9999999

	// defaultChunkLoaderCount is the number of chunk loaders to create
	defaultChunkLoaderCount = 10

	// Ticking intervals
	serverUpdateInterval  = 10 // Update servers every 10 ticks
	slapperUpdateInterval = 5  // Update slappers every 5 ticks
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
func NewPokeBedrock(log *slog.Logger, conf *Config) (*PokeBedrock, error) {
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

	// Initialize rank system with configuration
	rank.InitializeRanks(&rank.Config{
		TrainerRoleID:              conf.Ranks.TrainerRoleID,
		ServerBoosterRoleID:        conf.Ranks.ServerBoosterRoleID,
		SupporterRoleID:            conf.Ranks.SupporterRoleID,
		PremiumRoleID:              conf.Ranks.PremiumRoleID,
		ContentCreatorRoleID:       conf.Ranks.ContentCreatorRoleID,
		MonthlyTournamentMVPRoleID: conf.Ranks.MonthlyTournamentMVPRoleID,
		RetiredStaffRoleID:         conf.Ranks.RetiredStaffRoleID,
		HelperRoleID:               conf.Ranks.HelperRoleID,
		TeamRoleID:                 conf.Ranks.TeamRoleID,
		TranslatorRoleID:           conf.Ranks.TranslatorRoleID,
		DevelopmentTeamRoleID:      conf.Ranks.DevelopmentTeamRoleID,
		TrailModelerRoleID:         conf.Ranks.TrailModelerRoleID,
		ModelerRoleID:              conf.Ranks.ModelerRoleID,
		HeadModelerRoleID:          conf.Ranks.HeadModelerRoleID,
		ModeratorRoleID:            conf.Ranks.ModeratorRoleID,
		SeniorModeratorRoleID:      conf.Ranks.SeniorModeratorRoleID,
		HeadModeratorRoleID:        conf.Ranks.HeadModeratorRoleID,
		AdminRoleID:                conf.Ranks.AdminRoleID,
		ManagerRoleID:              conf.Ranks.ManagerRoleID,
		OwnerRoleID:                conf.Ranks.OwnerRoleID,
	})

	poke := &PokeBedrock{
		log:  log,
		conf: *conf,

		c:          make(chan struct{}),
		resManager: resManager,
	}
	go func() {
		if setupErr := poke.setupGin(); setupErr != nil {
			poke.log.Error("failed to start authentication service", "error", setupErr)
		}
	}()

	if err = poke.loadLocales(); err != nil {
		return nil, err
	}

	c.ReadOnlyWorld = true
	c.Generator = func(_ world.Dimension) world.Generator { // ensures that no new chunks are generated.
		return world.NopGenerator{}
	}
	c.StatusProvider = status.NewProvider(c.Name, c.Name) // ensures synchronized server count display.
	c.Allower = &Allower{}

	poke.srv = c.New()
	poke.srv.CloseOnProgramEnd()

	poke.loadCommands()
	poke.loadServices()

	return poke, nil
}

// Start begins the server's main loop, accepting connections and handling players.
// It blocks until the server is closed.
func (poke *PokeBedrock) Start() {
	poke.srv.Listen()
	poke.handleWorld()

	for pl := range poke.srv.Accept() {
		go poke.accept(pl)
	}

	poke.Close()
}

// handleWorld initializes and configures the world settings.
// It sets up the environment and starts background processes.
func (poke *PokeBedrock) handleWorld() {
	w := poke.World()

	l := world.NewLoader(defaultChunkLoaderCount, w, world.NopViewer{})
	w.Exec(func(tx *world.Tx) {
		l.Move(tx, w.Spawn().Vec3Middle())
		l.Load(tx, worldLoadRadius)
	})

	w.StopWeatherCycle()
	w.StopRaining()
	w.StopThundering()
	w.SetDefaultGameMode(world.GameModeAdventure)
	w.SetTime(defaultWorldTime)
	w.StopTime()
	w.SetTickRange(0)

	poke.loadServers()
	poke.loadSlappers()

	go poke.startTicking()
}

// setupGin sets up gin for the gobds proxy.
func (poke *PokeBedrock) setupGin() error {
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// Setup authentication route
	poke.setupAuthenticationRoute(router)

	// Setup restart manager routes
	poke.setupRestartRoutes(router)

	err := router.Run(poke.conf.Service.GinAddress)
	if err != nil {
		return err
	}

	poke.log.Info("Authentication service started on " + poke.conf.Service.GinAddress)
	return nil
}

// setupAuthenticationRoute sets up the authentication endpoint
func (poke *PokeBedrock) setupAuthenticationRoute(router *gin.Engine) {
	router.GET("/authentication/:xuid", func(c *gin.Context) {
		if !poke.validateAuth(c) {
			return
		}

		xuid := c.Param("xuid")
		if xuid == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "xuid is required"})
			return
		}

		poke.handleAuthenticationRequest(c, xuid)
	})
}

// setupRestartRoutes sets up all restart manager endpoints
func (poke *PokeBedrock) setupRestartRoutes(router *gin.Engine) {
	restartGroup := router.Group("/restart")
	{
		restartGroup.POST("/request", poke.handleRestartRequest)
		restartGroup.POST("/complete", poke.handleRestartComplete)
		restartGroup.POST("/unauthorized", poke.handleUnauthorizedRestart)
		restartGroup.GET("/state", poke.handleRestartState)
	}
}

// validateAuth validates the authorization header
func (poke *PokeBedrock) validateAuth(c *gin.Context) bool {
	if c.GetHeader("authorization") != poke.conf.Service.GinAuthenticationKey {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return false
	}
	return true
}

// handleAuthenticationRequest handles the authentication request logic
func (poke *PokeBedrock) handleAuthenticationRequest(c *gin.Context, xuid string) {
	req, exists := authentication.GlobalFactory().Of(xuid)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"reason": "no player found"})
		return
	}

	if time.Now().After(req.Expiration) {
		authentication.GlobalFactory().Remove(xuid)
		c.JSON(http.StatusGone, gin.H{"reason": "request expired"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"allowed": true})
}

// handleRestartRequest handles restart permission requests
func (poke *PokeBedrock) handleRestartRequest(c *gin.Context) {
	if !poke.validateAuth(c) {
		return
	}

	var req restart.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format", "details": err.Error()})
		return
	}

	response := restart.GlobalService().RequestRestart(req)
	c.JSON(http.StatusOK, response)
}

// handleRestartComplete handles restart completion notifications
func (poke *PokeBedrock) handleRestartComplete(c *gin.Context) {
	if !poke.validateAuth(c) {
		return
	}

	var notification restart.Notification
	if err := c.ShouldBindJSON(&notification); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification format", "details": err.Error()})
		return
	}

	if err := restart.GlobalService().NotifyRestartComplete(notification); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "acknowledged"})
}

// handleUnauthorizedRestart handles unauthorized restart notifications
func (poke *PokeBedrock) handleUnauthorizedRestart(c *gin.Context) {
	if !poke.validateAuth(c) {
		return
	}

	var req restart.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format", "details": err.Error()})
		return
	}

	restart.GlobalService().NotifyUnauthorizedRestart(req.ServerName)
	c.JSON(http.StatusOK, gin.H{"status": "acknowledged"})
}

// handleRestartState handles restart state queries
func (poke *PokeBedrock) handleRestartState(c *gin.Context) {
	if !poke.validateAuth(c) {
		return
	}

	stateJSON, err := restart.GlobalService().GetStateJSON()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get state", "details": err.Error()})
		return
	}

	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, string(stateJSON))
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
	vpn.NewService(poke.log, poke.conf.Service.VpnURL, poke.conf.Service.VpnCachePath)

	// Initialize restart manager service
	restartConfig := restart.Config{
		MaxWaitTime:     time.Duration(poke.conf.RestartManager.MaxWaitTime),
		BackoffInterval: time.Duration(poke.conf.RestartManager.BackoffInterval),
		RestartCooldown: time.Duration(poke.conf.RestartManager.RestartCooldown),
		QueueTimeout:    time.Duration(poke.conf.RestartManager.QueueTimeout),
		MaxRestartTime:  time.Duration(poke.conf.RestartManager.MaxRestartTime),
	}
	restart.NewService(poke.log, restartConfig)
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
		slapper.SummonAll(cfgs, tx, poke.resManager)
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

				switch {
				case f(serverUpdateInterval):
					srv.UpdateAll()
				case f(slapperUpdateInterval):
					slapper.UpdateAll(tx)
				case f(1):
					poke.doAFKCheck(tx)
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

	// We must exec this in a world transaction to ensure HandleJoin is called in the correct world.
	p.H().ExecWorld(func(tx *world.Tx, e world.Entity) {
		if p, ok := e.(*player.Player); ok {
			h.HandleJoin(p, tx.World())
		}
	})
}

// World returns the default world.
func (poke *PokeBedrock) World() *world.World {
	return poke.srv.World()
}

// doAFKCheck ...
func (poke *PokeBedrock) doAFKCheck(tx *world.Tx) {
	for ent := range tx.Players() {
		p := ent.(*player.Player)

		h, ok := p.Handler().(*handler.PlayerHandler)
		if !ok {
			continue
		}

		m := h.Movement()
		if time.Since(m.LastMoveTime()) > time.Duration(poke.conf.PokeBedrock.AFKTimeout) {
			p.Disconnect(text.Colourf("<red>You've been kicked for being AFK.</red>"))
		}
	}
}

// Close closes the server and all its associated services.
func (poke *PokeBedrock) Close() {
	poke.log.Debug("Closing Moderation Service...")
	moderation.GlobalService().Stop()

	poke.log.Debug("Closing Rank Service...")
	rank.GlobalService().Stop()

	poke.log.Debug("Closing Vpn Service...")
	vpn.GlobalService().Stop()

	poke.log.Debug("Closing Restart Manager Service...")
	restart.GlobalService().Stop()

	poke.log.Debug("Stopping Rank Channel...")
	session.StopRankChannel()

	poke.log.Debug("Stopping Rank Load Worker...")
	session.StopRankLoadWorker()

	poke.log.Debug("Stopping Infliction Worker...")
	session.StopInflictionWorker()

	close(poke.c)
}
