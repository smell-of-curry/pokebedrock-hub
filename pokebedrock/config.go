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

// Config holds the server configuration, including paths, translations, and service-related settings.
type Config struct {
	PokeBedrock struct {
		SentryDsn   string
		LogLevel    string // Can be "debug", "info", "warn", "error"
		ServerPath  string
		SlapperPath string
		LocalePath  string
		AFKTimeout  util.Duration
	}
	Service struct {
		GinAddress string

		RolesURL      string
		ModerationURL string
		ModerationKey string
		VpnURL        string

		AuthenticationPrefix string
		AuthenticationKey    string
	}
	RestartManager struct {
		MaxWaitTime     util.Duration
		MaxFailures     int
		BackoffInterval util.Duration
		RestartCooldown util.Duration
		QueueTimeout    util.Duration
	}
	server.UserConfig
}

// DefaultConfig returns a config with prefilled default values.
func DefaultConfig() Config {
	c := Config{}

	c.PokeBedrock.SentryDsn = ""
	c.PokeBedrock.LogLevel = "info" // Default to info level in production
	c.PokeBedrock.ServerPath = "resources/servers"
	c.PokeBedrock.SlapperPath = "resources/slapper"
	c.PokeBedrock.AFKTimeout = util.Duration(10 * time.Minute)

	c.Service.GinAddress = ":8080"

	c.Service.RolesURL = "http://127.0.0.1:4000/api/roles"
	c.Service.ModerationURL = "http://127.0.0.1:4000/api/moderation"
	c.Service.ModerationKey = "secret-key"
	c.Service.VpnURL = "http://ip-api.com/json"

	c.Service.AuthenticationPrefix = "authentication"
	c.Service.AuthenticationKey = "secret-key"

	c.RestartManager.MaxWaitTime = util.Duration(10 * time.Minute)
	c.RestartManager.MaxFailures = 3
	c.RestartManager.BackoffInterval = util.Duration(1 * time.Minute)
	c.RestartManager.RestartCooldown = util.Duration(5 * time.Minute)
	c.RestartManager.QueueTimeout = util.Duration(15 * time.Minute)

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
