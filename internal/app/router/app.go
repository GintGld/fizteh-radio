package router

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/storage/sqlite"

	authSrv "github.com/GintGld/fizteh-radio/internal/service/auth"
	djSrv "github.com/GintGld/fizteh-radio/internal/service/autodj"
	contentSrv "github.com/GintGld/fizteh-radio/internal/service/content"
	dashSrv "github.com/GintGld/fizteh-radio/internal/service/dash"
	jwtSrv "github.com/GintGld/fizteh-radio/internal/service/jwt"
	liveSrv "github.com/GintGld/fizteh-radio/internal/service/live"
	manSrv "github.com/GintGld/fizteh-radio/internal/service/manifest"
	mediaSrv "github.com/GintGld/fizteh-radio/internal/service/media"
	rootSrv "github.com/GintGld/fizteh-radio/internal/service/root"
	schSrv "github.com/GintGld/fizteh-radio/internal/service/schedule"
	srcSrv "github.com/GintGld/fizteh-radio/internal/service/source"
	statSrv "github.com/GintGld/fizteh-radio/internal/service/stat"

	authCtr "github.com/GintGld/fizteh-radio/internal/controller/auth"
	dashCtr "github.com/GintGld/fizteh-radio/internal/controller/dash"
	jwtCtr "github.com/GintGld/fizteh-radio/internal/controller/jwt"
	mediaCtr "github.com/GintGld/fizteh-radio/internal/controller/media"
	rootCtr "github.com/GintGld/fizteh-radio/internal/controller/root"
	schCtr "github.com/GintGld/fizteh-radio/internal/controller/schedule"
	statCtr "github.com/GintGld/fizteh-radio/internal/controller/stat"
)

type App struct {
	log     *slog.Logger
	address string
	app     *fiber.App
	dash    *dashSrv.Dash
}

// New returns configured router.App
func New(
	log *slog.Logger,
	storage *sqlite.Storage,
	address string,
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
	listenerTimeout time.Duration,
) *App {
	// Create sevices
	jwt := jwtSrv.New(secret)

	rootPassHash, err := bcrypt.GenerateFromPassword(rootPass, bcrypt.DefaultCost)
	if err != nil {
		panic("invalid root password")
	}

	sch2dashChan := make(chan models.Segment)
	sch2djChan := make(chan struct{})
	lib2djChan := make(chan struct{})

	// Authentication service
	auth := authSrv.New(
		log,
		storage,
		jwt,
		rootPassHash,
		tokenTTL,
	)
	// Root editor service
	root := rootSrv.New(
		log,
		storage,
	)
	// Media library service
	lib := mediaSrv.New(
		log,
		storage,
		maxAnswerLength,
		lib2djChan,
	)
	// Source library service
	src := srcSrv.New(
		log,
		sourceDir,
		nestingDepth,
		idLength,
	)
	// Schedule service
	sch := schSrv.New(
		log,
		storage,
		storage,
		sch2dashChan,
		sch2djChan,
	)
	// AutoDJ
	dj := djSrv.New(
		log,
		lib,
		sch,
		djCacheFile,
		sch2djChan,
		lib2djChan,
	)
	// Live streaming
	live := liveSrv.New(
		log,
		sch,
		liveDelay,
		liveStep,
		liveScript,
		contentDir,
		chunkLength,
	)
	// Dash manifest service
	man := manSrv.New(
		log,
		live,
		manPath,
		"http://"+address+"/radio/content",
		time.Now(),
		chunkLength,
		bufferTime,
		bufferDepth,
		clientUpdateFreq,
	)
	// Dash content generetor
	content := contentSrv.New(
		log,
		contentDir,
		chunkLength,
		lib,
		src,
	)
	// Dash goroutine
	dash := dashSrv.New(
		log,
		dashUpdateFreq,
		dashHorizon,
		man,
		content,
		sch,
		sch2dashChan,
	)
	// Stat
	stat := statSrv.New(
		log,
		storage,
		listenerTimeout,
	)

	// Controller helper
	jwtCtr := jwtCtr.New(secret)

	// TODO: body message limit more accurate

	// 300 MB limit for body message (~ 2.1 hours for .mp3 with 320 kbit/s)
	app := fiber.New(fiber.Config{
		BodyLimit: 300 * 1024 * 1024,
	})

	// Mount controllers to an app
	app.Mount("/login", authCtr.New(auth))
	app.Mount("/root", rootCtr.New(root, jwtCtr))
	app.Mount("/library", mediaCtr.New(lib, src, jwtCtr, tmpDir))
	app.Mount("/schedule", schCtr.New(sch, dj, live, jwtCtr))
	app.Mount("/radio", dashCtr.New(manPath, contentDir, jwtCtr, dash))
	app.Mount("/stat", statCtr.New(stat))

	// In debug mode there's no proxy that serves static files.
	if log.Enabled(context.Background(), slog.LevelDebug) {
		app.Static("/", "./public")

		app.Get("/mpd", func(c *fiber.Ctx) error {
			return c.SendFile(manPath)
		})
		app.Get("/:id/:file", func(c *fiber.Ctx) error {
			id := c.Params("id")
			if id == "" {
				return c.SendStatus(fiber.StatusNotFound)
			}

			file := c.Params("file")
			if file == "" {
				return c.SendStatus(fiber.StatusNotFound)
			}

			return c.SendFile(contentDir + "/" + id + "/" + file)
		})
	}

	if dashOnStart {
		go dash.Run(context.TODO())
	}
	if djOnStart {
		go dj.Run(context.TODO())
	}

	return &App{
		log:     log,
		address: address,
		app:     app,
		dash:    dash,
	}
}

func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		panic(err)
	}
}

func (a *App) Run() error {
	return a.app.Listen(a.address)
}

func (a *App) Stop() {
	go a.dash.Stop()
	a.app.Shutdown()
}
