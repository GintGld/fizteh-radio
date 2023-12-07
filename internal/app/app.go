package app

import (
	"log/slog"
	"os"
	"time"

	routerApp "github.com/GintGld/fizteh-radio/internal/app/router"
	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	"github.com/GintGld/fizteh-radio/internal/storage/sqlite"
)

type App struct {
	Router routerApp.App
}

func New(
	log *slog.Logger,
	address string,
	storagePath string,
	tokenTTL time.Duration,
	secret []byte,
	rootPass []byte,
) *App {
	storage, err := sqlite.New(storagePath)
	if err != nil {
		log.Error("failed to init storage", sl.Err(err))
		os.Exit(1)
	}

	routerApp := routerApp.New(
		log,
		storage,
		address,
		tokenTTL,
		secret,
		rootPass,
	)

	return &App{
		Router: *routerApp,
	}
}
