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

// TODO: Moderation api wrapper.

// main ...
func main() {
	log := slog.Default()
	conf, err := pokebedrock.ReadConfig()
	if err != nil {
		panic(err)
	}

	poke := pokebedrock.New(log, conf)
	if err = poke.Start(); err != nil {
		panic(err)
	}
}
