package config

import (
	"flag"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env           string        `yaml:"env" env-required:"true"`
	StoragePath   string        `yaml:"storage_path" env-required:"true"`
	TokenTTL      time.Duration `yaml:"token_ttl" env-default:"1h"`
	HTTPServer    `yaml:"http_server"`
	SourceStorage `yaml:"source_storage"`
	Dash          `yaml:"dash"`
}

type HTTPServer struct {
	Address      string        `yaml:"address" env-default:"localhost:8080"`
	Timeout      time.Duration `yaml:"timeout" end-default:"4s"`
	IddleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
	TmpDir       string        `yaml:"tmp_dir" env-default:"./tmp"`
}

type Dash struct {
	ManifestPath     string        `yaml:"manifest_path" env-required:"true"`
	ContentDir       string        `yaml:"content_dir" env-required:"true"`
	ChunkLength      time.Duration `yaml:"chunk_length" env-default:"2s"`
	BufferTime       time.Duration `yaml:"buffer_time" env-default:"30s"`
	BufferDepth      time.Duration `yaml:"buffer_depth" env-default:"5s"`
	ClientUpdateFreq time.Duration `yaml:"client_update_freq" env-default:"10s"`
	DashUpdateFreq   time.Duration `yaml:"dash_update_freq" env-default:"20s"`
	DashHorizon      time.Duration `yaml:"dash_horizon" env-default:"5m"`
}

type SourceStorage struct {
	SourcePath   string `yaml:"path" env-required:"true"`
	NestingDepth int    `yaml:"nesting_depth" env-required:"true"`
	IdLength     int    `yaml:"id_length" env-required:"true"`
}

func MustLoad() *Config {
	configPath := fetchConfigPath()
	if configPath == "" {
		panic("config path is empty")
	}

	return MustLoadPath(configPath)
}

func MustLoadPath(configPath string) *Config {
	// check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exist: " + configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic("cannot read config: " + err.Error())
	}

	return &cfg
}

// fetchConfigPath fetches config path from command line flag or environment variable.
// Priority: flag > env > default.
// Default value is empty string.
func fetchConfigPath() string {
	var res string

	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	return res
}
