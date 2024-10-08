package config

import (
	"flag"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env             string        `yaml:"env" env-required:"true"`
	LogPath         string        `yaml:"log_path" env-default:""`
	StoragePath     string        `yaml:"storage_path" env-required:"true"`
	TokenTTL        time.Duration `yaml:"token_ttl" env-default:"1h"`
	ListenerTimeout time.Duration `yaml:"listener_timeout" env-default:"2s"`
	HttpServer      HTTPServer    `yaml:"http_server"`
	Source          SourceStorage `yaml:"source_storage"`
	Dash            Dash          `yaml:"dash"`
	DJ              DJ            `yaml:"dj"`
	Live            Live          `yaml:"live"`
}

type HTTPServer struct {
	Address         string        `yaml:"address" env-default:"localhost:8080"`
	Timeout         time.Duration `yaml:"timeout" end-default:"4s"`
	IddleTimeout    time.Duration `yaml:"idle_timeout" env-default:"60s"`
	MaxAnswerLength int           `yaml:"max-answer-length" env-default:"100"`
	TmpDir          string        `yaml:"tmp_dir" env-default:"./tmp"`
}

type Dash struct {
	DashOnStart      bool          `yaml:"dash_on_start" env-default:"false"`
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
	Addr       string        `yaml:"addr" env-required:"true"`
	Timeout    time.Duration `yaml:"timeout" env-default:"30s"`
	RetryCount int           `yaml:"retry" env-default:"5"`
}

type DJ struct {
	DjOnStart   bool   `yaml:"dj_on_start" env-default:"false"`
	DjCacheFile string `yaml:"cache_file" env-required:"true"`
}

type Live struct {
	Delay        time.Duration     `yaml:"delay" env-default:"2s"`
	StepDuration time.Duration     `yaml:"step_duration" env-default:"5m"`
	SourceType   string            `yaml:"source-type" env-default:""`
	Source       string            `yaml:"source" env-required:"true"`
	Filters      map[string]string `yaml:"filters"`
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
