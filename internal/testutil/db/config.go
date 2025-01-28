package db

import (
	"path/filepath"
	"runtime"
	"testing"
	"wattwatch/internal/config"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

func LoadTestConfig(t *testing.T) *config.Config {
	t.Helper()

	// Get the absolute path to this file
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "Failed to get current file path")

	// Calculate project root (3 levels up from this file)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..")
	projectRoot, err := filepath.Abs(projectRoot)
	require.NoError(t, err, "Failed to get absolute project root path")

	err = godotenv.Load(filepath.Join(projectRoot, ".env.test"))
	require.NoError(t, err, "Failed to load .env.test file")

	// Create a new config instance
	cfg := &config.Config{}

	// Load from environment
	err = cfg.LoadFromEnv()
	require.NoError(t, err, "Failed to load config")

	// Only override migrations path to ensure it's absolute
	cfg.Database.MigrationsPath = filepath.Join(projectRoot, "migrations")

	return cfg
}
