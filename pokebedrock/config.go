package pokebedrock

import (
	"os"

	"github.com/df-mc/dragonfly/server"
	"github.com/restartfu/gophig"
	"github.com/sandertv/gophertunnel/minecraft/text"
)

// Config ...
type Config struct {
	PokeBedrock struct {
		ServerPath  string
		SlapperPath string
	}
	server.UserConfig
}

// DefaultConfig ...
func DefaultConfig() Config {
	c := Config{}

	c.PokeBedrock.ServerPath = "resources/servers"
	c.PokeBedrock.SlapperPath = "resources/slapper"

	userConfig := server.DefaultConfig()
	userConfig.Server.Name = text.Colourf("<red>Poke</red><aqua>Bedrock</aqua>")
	userConfig.Server.DisableJoinQuitMessages = true
	userConfig.Players.Folder = "resources/player_data"
	userConfig.Resources.Folder = "resources/resource_pack"
	userConfig.World.Folder = "resources/world"
	c.UserConfig = userConfig

	return c
}

// ReadConfig ...
func ReadConfig() (Config, error) {
	g := gophig.NewGophig[Config]("./config.toml", gophig.TOMLMarshaler{}, os.ModePerm)
	c, err := g.LoadConf()
	if os.IsNotExist(err) {
		err = g.SaveConf(DefaultConfig())
		if err != nil {
			return Config{}, err
		}
	}
	c, err = g.LoadConf()
	return c, err
}
