// Package main is the entry point for the application.
package main

import (
	"log/slog"
	"time"

	"github.com/df-mc/dragonfly/server/player/chat"
	"github.com/getsentry/sentry-go"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock"
)

// init sets up basic logging until config is loaded
func init() {
	// Default to debug during startup, will be reconfigured after config load
	slog.SetLogLoggerLevel(slog.LevelDebug)
	chat.Global.Subscribe(chat.StdoutSubscriber{})
}

// main is the entry point for the application. It initializes the configuration,
// sets the appropriate log level, creates the PokeBedrock server instance,
// and starts it.
func main() {
	log := slog.Default()

	conf, err := pokebedrock.ReadConfig()
	if err != nil {
		panic(err)
	}

	err = sentry.Init(sentry.ClientOptions{
		Dsn: conf.PokeBedrock.SentryDsn,
	})
	if err != nil {
		log.Error("sentry.Init", "error", err)
		panic(err)
	}

	// Flush buffered events before the program terminates.
	defer sentry.Flush(2 * time.Second)

	// Set the log level based on configuration
	logLevel, err := pokebedrock.ParseLogLevel(conf.PokeBedrock.LogLevel)
	if err != nil {
		log.Warn("Invalid log level in configuration", "error", err, "using", "info")

		logLevel = slog.LevelInfo
	}

	slog.SetLogLoggerLevel(logLevel)
	log.Info("Log level set", "level", logLevel.String())

	poke, err := pokebedrock.NewPokeBedrock(log, conf)
	if err != nil {
		panic(err)
	}

	poke.Start()
}
