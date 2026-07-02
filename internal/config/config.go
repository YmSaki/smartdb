package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port           int
	DataDir        string
	LogLevel       string
	QueryTimeout   time.Duration
	CORSOrigins    string
	BackupInterval time.Duration
	BackupMaxGen   int
	SystemToken    string
	MaxBodyBytes   int64
}

// LoadDefaults returns a Config with default values, ignoring environment variables.
func LoadDefaults() *Config {
	return &Config{
		Port:           8080,
		DataDir:        "./data",
		LogLevel:       "info",
		QueryTimeout:   5 * time.Second,
		CORSOrigins:    "*",
		BackupInterval: 24 * time.Hour,
		BackupMaxGen:   7,
		MaxBodyBytes:   1 << 20, // 1MB, per spec.md §11
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

	if v := os.Getenv("SDB_CORS_ORIGINS"); v != "" {
		cfg.CORSOrigins = v
	}

	if v := os.Getenv("SDB_BACKUP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.BackupInterval = d
		}
	}

	if v := os.Getenv("SDB_BACKUP_MAX_GEN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.BackupMaxGen = n
		}
	}

	if v := os.Getenv("SDB_SYSTEM_TOKEN"); v != "" {
		cfg.SystemToken = v
	}

	if v := os.Getenv("SDB_MAX_BODY_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxBodyBytes = n
		}
	}

	return cfg
}
