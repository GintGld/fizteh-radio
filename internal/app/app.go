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
	Router  routerApp.App
	Storage *sqlite.Storage
}

func New(
	log *slog.Logger,
	address string,
	storagePath string,
	timeout time.Duration,
	idleTimeout time.Duration,
	tokenTTL time.Duration,
	secret []byte,
	rootPass []byte,
	maxAnswerLength int,
	tmpDir string,
	sourceAddr string,
	sourceTimeout time.Duration,
	sourceRetryCount int,
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
	liveSourceType string,
	liveSource string,
	liveFilters map[string]string,
	listenerTimeout time.Duration,
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
		timeout,
		idleTimeout,
		tokenTTL,
		secret,
		rootPass,
		maxAnswerLength,
		tmpDir,
		sourceAddr,
		sourceTimeout,
		sourceRetryCount,
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
		liveSourceType,
		liveSource,
		liveFilters,
		listenerTimeout,
	)

	return &App{
		Router:  *routerApp,
		Storage: storage,
	}
}
