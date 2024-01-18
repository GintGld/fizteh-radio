package main

import (
	"io"
	"os"
	"os/signal"
	"syscall"

	"log/slog"

	"github.com/GintGld/fizteh-radio/internal/app"
	"github.com/GintGld/fizteh-radio/internal/config"
	"github.com/GintGld/fizteh-radio/internal/lib/logger/slogpretty"
)

const (
	envLocal = "local"
	envProd  = "prod"
)

func main() {
	cfg := config.MustLoad()

	log := setupLogger(cfg.Env, cfg.LogPath)

	log.Info("starting radio", slog.String("env", cfg.Env))
	log.Debug("debug messages are enabled")

	// TODO: send timeout and iddletimeout
	httpApplication := app.New(
		log,
		cfg.Address,
		cfg.StoragePath,
		cfg.TokenTTL,
		getSecret(),
		getRootPass(),
		cfg.TmpDir,
		cfg.SourcePath,
		cfg.NestingDepth,
		cfg.IdLength,
		cfg.ManifestPath,
		cfg.ContentDir,
		cfg.ChunkLength,
		cfg.BufferTime,
		cfg.BufferDepth,
		cfg.ClientUpdateFreq,
		cfg.DashUpdateFreq,
		cfg.DashHorizon,
	)

	// Run server
	go func() {
		httpApplication.Router.MustRun()
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	<-stop

	httpApplication.Router.Stop()
	log.Info("Gracefully stopped")
}

func setupLogger(env, logPath string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = setupPrettySlog()
	case envProd:
		var logWriter io.Writer

		if logPath == "" {
			logWriter = os.Stdout
		} else {
			var err error
			logWriter, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				panic("failed to open log file. Error: " + err.Error())
			}
		}

		log = slog.New(
			slog.NewJSONHandler(logWriter, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}

func setupPrettySlog() *slog.Logger {
	opts := slogpretty.PrettyHandlerOptions{
		SlogOpts: &slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}

	handler := opts.NewPrettyHandler(os.Stdout)

	return slog.New(handler)
}

func getSecret() []byte {
	secret := os.Getenv("SECRET")

	if secret == "" {
		panic("secret not specified")
	}

	return []byte(secret)
}

func getRootPass() []byte {
	pass := os.Getenv("ROOT_PASS")

	if pass == "" {
		panic("root passwrod is not specified")
	}

	return []byte(pass)
}
