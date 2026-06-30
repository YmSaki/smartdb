package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port         int
	DataDir      string
	LogLevel     string
	QueryTimeout time.Duration
}

// LoadDefaults returns a Config with default values, ignoring environment variables.
func LoadDefaults() *Config {
	return &Config{
		Port:         8080,
		DataDir:      "./data",
		LogLevel:     "info",
		QueryTimeout: 5 * time.Second,
	}
}

// Load reads configuration from environment variables with defaults.
func Load() *Config {
	cfg := LoadDefaults()

	if v := os.Getenv("SDB_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}

	if v := os.Getenv("SDB_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	if v := os.Getenv("SDB_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	if v := os.Getenv("SDB_QUERY_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.QueryTimeout = d
		}
	}

	return cfg
}
