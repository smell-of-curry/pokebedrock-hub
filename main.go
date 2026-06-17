// Package main is the entry point for the application.
package main

import (
	"log/slog"
	"net/http"
	_ "net/http/pprof" // registers /debug/pprof handlers on http.DefaultServeMux
	"time"

	"github.com/df-mc/dragonfly/server/player/chat"
	"github.com/getsentry/sentry-go"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock"
)

// pprofAddress is the localhost-only address the debug/pprof server listens on.
// It is bound to loopback so profiles are only reachable on the host (use an
// SSH tunnel, e.g. `ssh -L 6060:localhost:6060 <host>`, to read them remotely).
const pprofAddress = "localhost:6060"

// init sets up basic logging until config is loaded
func init() {
	// Default to debug during startup, will be reconfigured after config load
	slog.SetLogLoggerLevel(slog.LevelDebug)
	chat.Global.Subscribe(chat.StdoutSubscriber{})
}

// startPprof starts the localhost-only pprof HTTP server used to capture
// goroutine and heap profiles while the hub is running. It runs until the
// process exits.
//
// @param log The logger used to report the listen address and any fatal error.
func startPprof(log *slog.Logger) {
	go func() {
		log.Info("pprof debug server listening", "addr", pprofAddress)
		if err := http.ListenAndServe(pprofAddress, nil); err != nil {
			log.Error("pprof debug server stopped", "error", err)
		}
	}()
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

	startPprof(log)

	poke, err := pokebedrock.NewPokeBedrock(log, conf)
	if err != nil {
		panic(err)
	}

	poke.Start()
}
