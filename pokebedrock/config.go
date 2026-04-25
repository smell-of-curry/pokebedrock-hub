package pokebedrock

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/df-mc/dragonfly/server"
	"github.com/restartfu/gophig"
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

	c.Service.GinAddress = ":8080"

	c.Service.RolesURL = "http://127.0.0.1:4000/api/roles"
	c.Service.ModerationURL = "http://127.0.0.1:4000/api/moderation"
	c.Service.ModerationKey = "secret-key"

	// Default VPN cache path
	c.Service.VpnCachePath = "resources/vpnResults.json"
	c.Service.VpnURL = "http://ip-api.com/json"

	c.Service.GinAuthenticationKey = "secret-key"

	c.RestartManager.MaxWaitTime = util.Duration(defaultMaxWaitTime)
	c.RestartManager.BackoffInterval = util.Duration(defaultBackoffInterval)
	c.RestartManager.RestartCooldown = util.Duration(defaultRestartCooldown)
	c.RestartManager.QueueTimeout = util.Duration(defaultQueueTimeout)
	c.RestartManager.MaxRestartTime = util.Duration(defaultMaxRestartTime)

	c.Parkour.LeaderboardPath = "resources/parkour/leaderboard.json"
	c.Parkour.CountdownSeconds = defaultParkourCountdownSeconds
	c.Parkour.CompletionRadius = defaultParkourCompletionRadius

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
	g := gophig.NewGophig[Config]("./config.toml", gophig.TOMLMarshaler{}, os.ModePerm)

	_, err := g.LoadConf()
	if os.IsNotExist(err) {
		err = g.SaveConf(DefaultConfig())
		if err != nil {
			return Config{}, err
		}
	}

	return g.LoadConf()
}
