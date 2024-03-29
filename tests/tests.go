package tests

import (
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/GintGld/fizteh-radio/internal/config"
)

// Actual environment
var (
	_        = godotenv.Load("../.env")
	cfg      = config.MustLoadPath(os.Getenv("CONFIG_PATH"))
	rootPass = os.Getenv("ROOT_PASS")
	secret   = os.Getenv("SECRET")
)

var (
	sourceFile     = "./source/sample-9s.mp3"
	sourceDuration = time.Second * 9 // approximate duration
)

// TODO: try to use fiber.App.test method for testing
