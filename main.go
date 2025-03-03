package main

import (
	"log/slog"

	"github.com/df-mc/dragonfly/server/player/chat"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock"
)

// init ...
func init() {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	chat.Global.Subscribe(chat.StdoutSubscriber{})
}

// main ...
func main() {
	log := slog.Default()
	conf, err := pokebedrock.ReadConfig()
	if err != nil {
		panic(err)
	}

	poke, err := pokebedrock.NewPokeBedrock(log, conf)
	if err != nil {
		panic(err)
	}

	poke.Start()
}
