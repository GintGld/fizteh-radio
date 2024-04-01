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
	maxAnswerLength int,
	tmpDir string,
	sourceDir string,
	nestingDepth int,
	idLength int,
	manPath string,
	contentDir string,
	chunkLength time.Duration,
	bufferTime time.Duration,
	bufferDepth time.Duration,
	clientUpdateFreq time.Duration,
	dashUpdateFreq time.Duration,
	dashHorizon time.Duration,
	dashOnStart bool,
	djOnStart bool,
	djCacheFile string,
	liveDelay time.Duration,
	liveStep time.Duration,
	liveScript string,
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
		maxAnswerLength,
		tmpDir,
		sourceDir,
		nestingDepth,
		idLength,
		manPath,
		contentDir,
		chunkLength,
		bufferTime,
		bufferDepth,
		clientUpdateFreq,
		dashUpdateFreq,
		dashHorizon,
		dashOnStart,
		djOnStart,
		djCacheFile,
		liveDelay,
		liveStep,
		liveScript,
	)

	return &App{
		Router: *routerApp,
	}
}
