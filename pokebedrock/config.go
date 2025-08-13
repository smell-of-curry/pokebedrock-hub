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
	c.PokeBedrock.SlapperPath = "resources/slapper"
	c.PokeBedrock.AFKTimeout = util.Duration(10 * time.Minute)

	c.Service.GinAddress = ":8080"

	c.Service.RolesURL = "http://127.0.0.1:4000/api/roles"
	c.Service.ModerationURL = "http://127.0.0.1:4000/api/moderation"
	c.Service.ModerationKey = "secret-key"

	// Default VPN cache path
	c.Service.VpnCachePath = "resources/vpnResults.json"
	c.Service.VpnURL = "http://ip-api.com/json"

	c.Service.GinAuthenticationKey = "secret-key"

	c.RestartManager.MaxWaitTime = util.Duration(10 * time.Minute)
	c.RestartManager.BackoffInterval = util.Duration(1 * time.Minute)
	c.RestartManager.RestartCooldown = util.Duration(5 * time.Minute)
	c.RestartManager.QueueTimeout = util.Duration(15 * time.Minute)
	c.RestartManager.MaxRestartTime = util.Duration(20 * time.Minute)

	// Default Discord Role IDs (should be configured by users)
	c.Ranks.TrainerRoleID = "1068581342159306782"
	c.Ranks.ServerBoosterRoleID = "1068578576951160952"
	c.Ranks.SupporterRoleID = "1088998497061175296"
	c.Ranks.PremiumRoleID = "1096068279786815558"
	c.Ranks.ContentCreatorRoleID = "1084485790605787156"
	c.Ranks.MonthlyTournamentMVPRoleID = "1281044331121217538"
	c.Ranks.RetiredStaffRoleID = "1179937172455952384"
	c.Ranks.HelperRoleID = "1088902437093523566"
	c.Ranks.TeamRoleID = "1067977855700574238"
	c.Ranks.TranslatorRoleID = "1137751922217058365"
	c.Ranks.DevelopmentTeamRoleID = "1123082881380646944"
	c.Ranks.TrailModelerRoleID = "1085669665298194482"
	c.Ranks.ModelerRoleID = "1080719745290088498"
	c.Ranks.HeadModelerRoleID = "1085669297034117200"
	c.Ranks.ModeratorRoleID = "1083171623282163743"
	c.Ranks.SeniorModeratorRoleID = "1295545506646462504"
	c.Ranks.HeadModeratorRoleID = "1131819233874022552"
	c.Ranks.AdminRoleID = "1083171563798540349"
	c.Ranks.ManagerRoleID = "1067977172339396698"
	c.Ranks.OwnerRoleID = "1055833987739824258"

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
