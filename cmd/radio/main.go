package main

import (
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
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)

	log.Info("starting radio", slog.String("env", cfg.Env))
	log.Debug("debug messages are enabled")

	// TODO: init httpApplication: fiber
	httpApplication := app.New(
		log,
		cfg.Address,
		cfg.StoragePath,
		cfg.TokenTTL,
		getSecret(),   // TODO
		getRootPass(), // TODO
	)

	// TODO: init scheduler: DASH

	// TODO: run scheduler

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

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = setupPrettySlog()
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
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
