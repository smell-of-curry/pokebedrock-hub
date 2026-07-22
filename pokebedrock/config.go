package pokebedrock

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/df-mc/dragonfly/server"
	"github.com/restartfu/gophig"
	"github.com/restartfu/gophig/codecs"
	"github.com/sandertv/gophertunnel/minecraft/text"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/util"
)

const (
	// Default timeout and duration constants
	defaultAFKTimeout         = 10 * time.Minute
	defaultAFKWarnApproaching = 4 * time.Minute
	defaultAFKMarkAFK         = 5 * time.Minute
	defaultAFKFinalWarning    = 9 * time.Minute
	defaultAFKFullnessThresh  = 0.90
	defaultMaxWaitTime        = 10 * time.Minute
	defaultBackoffInterval    = 1 * time.Minute
	defaultQueueTimeout       = 15 * time.Minute
	defaultMaxRestartTime     = 20 * time.Minute
	defaultRestartCooldown    = 5 * time.Minute

	defaultParkourCountdownSeconds = 5
	defaultParkourCompletionRadius = 1.25

	defaultWatchdogCheckInterval      = 30 * time.Second
	defaultWatchdogWorldExecTimeout   = 10 * time.Second
	defaultWatchdogGoroutineThreshold = 800
	defaultWatchdogHeapAllocThreshold = 2 << 30 // 2 GiB
	defaultWatchdogAlertCooldown      = 5 * time.Minute

	defaultDevServersPollIntervalSeconds = 10
)

// Config holds the server configuration, including paths, translations, and service-related settings.
type Config struct {
	PokeBedrock struct {
		SentryDsn  string
		LogLevel   string // Can be "debug", "info", "warn", "error"
		ServerPath string
		LocalePath string
		// AFKTimeout is how long a player must be idle before becoming
		// eligible to be kicked for being AFK. The kick itself only fires
		// when the hub is at or above AFKFullnessThreshold full.
		AFKTimeout util.Duration
		// AFKWarnApproaching is how long a player must be idle before
		// receiving the "AFK in 1 minute" soft warning. Always sent.
		AFKWarnApproaching util.Duration
		// AFKMarkAFK is how long a player must be idle before being told
		// they are now AFK. Always sent.
		AFKMarkAFK util.Duration
		// AFKFinalWarning is how long a player must be idle before
		// receiving the near-capacity hard warning. Only sent when fullness
		// >= AFKFullnessThreshold.
		AFKFinalWarning util.Duration
		// AFKFullnessThreshold is the fraction (0..1) of the hub's MaxCount
		// at or above which AFK players will start getting kicked,
		// longest-AFK first.
		AFKFullnessThreshold float64
		// DowntimeLock blocks non–Sr. Moderator players from joining downstream
		// servers while the network is in downtime. The hub itself stays open.
		DowntimeLock bool
	}
	Service struct {
		GinAddress           string
		GinAuthenticationKey string

		RolesURL      string
		ModerationURL string
		ModerationKey string

		// VpnCachePath is the file path used to persist VPN IP results.
		// Defaults to resources/vpnResults.json
		VpnCachePath string
		VpnURL       string
		// VpnWhitelist is a list of CIDR ranges (e.g. "45.230.64.0/22")
		// that are never treated as VPN/proxy connections. Used for
		// residential ISP blocks the detection API misclassifies.
		VpnWhitelist []string
	}
	RestartManager struct {
		MaxWaitTime     util.Duration
		BackoffInterval util.Duration
		RestartCooldown util.Duration
		QueueTimeout    util.Duration
		MaxRestartTime  util.Duration
	}
	Parkour struct {
		LeaderboardPath  string
		CountdownSeconds int
		CompletionRadius float64
	}
	Watchdog struct {
		// CheckInterval is how often the health watchdog probes the world tick
		// and process metrics.
		CheckInterval util.Duration
		// WorldExecTimeout is how long a probe world owner task may take
		// before the world is considered stalled (login-blocking deadlock).
		// Name retained for deployed config.toml compatibility.
		WorldExecTimeout util.Duration
		// GoroutineThreshold is the goroutine count at/above which an alert is
		// raised.
		GoroutineThreshold int
		// HeapAllocThresholdBytes is the heap-allocated byte count at/above
		// which an alert is raised. 0 disables the heap probe.
		HeapAllocThresholdBytes uint64
		// AlertCooldown is the minimum time between repeat Sentry alerts for
		// the same condition.
		AlertCooldown util.Duration
	}
	Ranks struct {
		TrainerRoleID              string
		ServerBoosterRoleID        string
		SupporterRoleID            string
		PremiumRoleID              string
		ContentCreatorRoleID       string
		MonthlyTournamentMVPRoleID string
		RetiredStaffRoleID         string
		HelperRoleID               string
		TeamRoleID                 string
		TranslatorRoleID           string
		DevelopmentTeamRoleID      string
		TrailModelerRoleID         string
		ModelerRoleID              string
		HeadModelerRoleID          string
		ModeratorRoleID            string
		SeniorModeratorRoleID      string
		HeadModeratorRoleID        string
		AdminRoleID                string
		ManagerRoleID              string
		OwnerRoleID                string
	}
	DevServers struct {
		// Enabled turns on polling the remote manager for beta/dev servers.
		Enabled bool
		// URL is the manager API base (e.g. https://dev.pokebedrock.com).
		URL string
		// Token is sent as the authorization header.
		Token string
		// Host is the public IP/host the managed servers listen on.
		Host string
		// PollIntervalSeconds is how often to GET /dev-servers.
		PollIntervalSeconds int
	}
	server.UserConfig
}

// DefaultConfig returns a config with prefilled default values.
func DefaultConfig() Config {
	c := Config{}

	c.PokeBedrock.SentryDsn = ""
	c.PokeBedrock.LogLevel = "info" // Default to info level in production
	c.PokeBedrock.ServerPath = "resources/servers"
	c.PokeBedrock.AFKTimeout = util.Duration(defaultAFKTimeout)
	c.PokeBedrock.AFKWarnApproaching = util.Duration(defaultAFKWarnApproaching)
	c.PokeBedrock.AFKMarkAFK = util.Duration(defaultAFKMarkAFK)
	c.PokeBedrock.AFKFinalWarning = util.Duration(defaultAFKFinalWarning)
	c.PokeBedrock.AFKFullnessThreshold = defaultAFKFullnessThresh
	c.PokeBedrock.DowntimeLock = false

	c.Service.GinAddress = ":8080"

	c.Service.RolesURL = "http://127.0.0.1:4000/api/roles"
	c.Service.ModerationURL = "http://127.0.0.1:4000/api/moderation"
	c.Service.ModerationKey = "secret-key"

	// Default VPN cache path
	c.Service.VpnCachePath = "resources/vpnResults.json"
	c.Service.VpnURL = "http://ip-api.com/json"
	// Megalink S.R.L. (Argentina) — residential ISP flagged as proxy by ip-api.
	c.Service.VpnWhitelist = []string{"45.230.64.0/22"}

	c.Service.GinAuthenticationKey = "secret-key"

	c.RestartManager.MaxWaitTime = util.Duration(defaultMaxWaitTime)
	c.RestartManager.BackoffInterval = util.Duration(defaultBackoffInterval)
	c.RestartManager.RestartCooldown = util.Duration(defaultRestartCooldown)
	c.RestartManager.QueueTimeout = util.Duration(defaultQueueTimeout)
	c.RestartManager.MaxRestartTime = util.Duration(defaultMaxRestartTime)

	c.Parkour.LeaderboardPath = "resources/parkour/leaderboard.json"
	c.Parkour.CountdownSeconds = defaultParkourCountdownSeconds
	c.Parkour.CompletionRadius = defaultParkourCompletionRadius

	c.Watchdog.CheckInterval = util.Duration(defaultWatchdogCheckInterval)
	c.Watchdog.WorldExecTimeout = util.Duration(defaultWatchdogWorldExecTimeout)
	c.Watchdog.GoroutineThreshold = defaultWatchdogGoroutineThreshold
	c.Watchdog.HeapAllocThresholdBytes = defaultWatchdogHeapAllocThreshold
	c.Watchdog.AlertCooldown = util.Duration(defaultWatchdogAlertCooldown)

	c.DevServers.Enabled = false
	c.DevServers.URL = ""
	c.DevServers.Token = ""
	c.DevServers.Host = ""
	c.DevServers.PollIntervalSeconds = defaultDevServersPollIntervalSeconds

	userConfig := server.DefaultConfig()
	userConfig.Server.Name = text.Colourf("<red>Poke</red><aqua>Bedrock</aqua>")
	userConfig.World.Folder = "resources/world"

	userConfig.Players.Folder = "resources/player_data"
	userConfig.Players.MaximumChunkRadius = 8

	userConfig.Resources.Required = true
	userConfig.Resources.Folder = "resources/resource_pack"
	userConfig.Resources.AutoBuildPack = false

	c.UserConfig = userConfig

	return c
}

// ParseLogLevel returns the appropriate slog.Level based on string configuration.
// Returns an error if the provided log level string is not recognized.
func ParseLogLevel(level string) (slog.Level, error) {
	switch level {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unrecognized log level: %q", level)
	}
}

// ReadConfig loads the server configuration from config.toml.
// If the file doesn't exist, it creates a new one with default values.
// Returns the loaded configuration and any error encountered.
func ReadConfig() (Config, error) {
	g := gophig.NewGophig[Config]("./config.toml", codecs.TOMLMarshaler{}, os.ModePerm)

	conf, err := g.LoadConf()
	if os.IsNotExist(err) {
		conf = DefaultConfig()
		if err = g.SaveConf(conf); err != nil {
			return Config{}, err
		}
		return conf, nil
	}
	if err != nil {
		return Config{}, err
	}

	// gophig unmarshals into a zero Config; missing TOML keys stay zero.
	// Overlay defaults for unset watchdog durations so startup/world probes
	// do not get context.WithTimeout(..., 0) → immediate deadline exceeded.
	defaults := DefaultConfig()
	if conf.Watchdog.CheckInterval == 0 {
		conf.Watchdog.CheckInterval = defaults.Watchdog.CheckInterval
	}
	if conf.Watchdog.WorldExecTimeout == 0 {
		conf.Watchdog.WorldExecTimeout = defaults.Watchdog.WorldExecTimeout
	}
	if conf.Watchdog.AlertCooldown == 0 {
		conf.Watchdog.AlertCooldown = defaults.Watchdog.AlertCooldown
	}
	if conf.Watchdog.GoroutineThreshold == 0 {
		conf.Watchdog.GoroutineThreshold = defaults.Watchdog.GoroutineThreshold
	}
	if conf.Watchdog.HeapAllocThresholdBytes == 0 {
		conf.Watchdog.HeapAllocThresholdBytes = defaults.Watchdog.HeapAllocThresholdBytes
	}
	if conf.DevServers.PollIntervalSeconds == 0 {
		conf.DevServers.PollIntervalSeconds = defaults.DevServers.PollIntervalSeconds
	}

	return conf, nil
}
