package router

import (
	"log/slog"
	"time"

	"github.com/GintGld/fizteh-radio/internal/storage/sqlite"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	authSrv "github.com/GintGld/fizteh-radio/internal/service/auth"
	contentSrv "github.com/GintGld/fizteh-radio/internal/service/content"
	dashSrv "github.com/GintGld/fizteh-radio/internal/service/dash"
	jwtSrv "github.com/GintGld/fizteh-radio/internal/service/jwt"
	manSrv "github.com/GintGld/fizteh-radio/internal/service/manifest"
	mediaSrv "github.com/GintGld/fizteh-radio/internal/service/media"
	rootSrv "github.com/GintGld/fizteh-radio/internal/service/root"
	schSrv "github.com/GintGld/fizteh-radio/internal/service/schedule"
	srcSrv "github.com/GintGld/fizteh-radio/internal/service/source"

	authCtr "github.com/GintGld/fizteh-radio/internal/controller/auth"
	dashCtr "github.com/GintGld/fizteh-radio/internal/controller/dash"
	jwtCtr "github.com/GintGld/fizteh-radio/internal/controller/jwt"
	mediaCtr "github.com/GintGld/fizteh-radio/internal/controller/media"
	rootCtr "github.com/GintGld/fizteh-radio/internal/controller/root"
	schCtr "github.com/GintGld/fizteh-radio/internal/controller/schedule"
)

type App struct {
	log     *slog.Logger
	address string
	app     *fiber.App
	dash    *dashSrv.Dash
}

// TODO: special handler (controller) to start/stop dash app

// New returns configured router.App
func New(
	log *slog.Logger,
	storage *sqlite.Storage,
	address string,
	tokenTTL time.Duration,
	secret []byte,
	rootPass []byte,
	tmpDir string,
	sourceDir string,
	nestingDepth int,
	manPath string,
	contentDir string,
	chunkLength time.Duration,
	bufferTime time.Duration,
	bufferDepth time.Duration,
	clientUpdateFreq time.Duration,
	dashUpdateFreq time.Duration,
	dashHorizon time.Duration,
) *App {
	// Create sevices
	jwt := jwtSrv.New(secret)

	rootPassHash, err := bcrypt.GenerateFromPassword(rootPass, bcrypt.DefaultCost)
	if err != nil {
		panic("invalid root password")
	}
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
	)
	// Source library service
	src := srcSrv.New(
		log,
		sourceDir,
		nestingDepth,
	)
	// Schedule service
	sch := schSrv.New(
		log,
		storage,
		storage,
	)
	// Dash manifest service
	man := manSrv.New(
		log,
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
	dash := dashSrv.New(
		log,
		dashUpdateFreq,
		dashHorizon,
		man,
		content,
		sch,
	)

	// Controller helper
	jwtCtr := jwtCtr.New(secret)

	// 300 MB limit for body message (~ 2.1 hours for .mp3 with 320 kbit/s)
	app := fiber.New(fiber.Config{
		BodyLimit: 300 * 1024 * 1024,
	})

	// Mount controllers to an app
	app.Mount("/login", authCtr.New(auth))
	app.Mount("/root", rootCtr.New(root, jwtCtr))
	app.Mount("/library", mediaCtr.New(lib, src, jwtCtr, tmpDir))
	app.Mount("/schedule", schCtr.New(sch, jwtCtr))
	app.Mount("/radio", dashCtr.New(manPath, contentDir, jwtCtr, dash))
	app.Static("/", "./public")

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
