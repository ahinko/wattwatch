package config

import (
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

// LoadTestConfig loads configuration for testing
func LoadTestConfig(t *testing.T) *Config {
	t.Helper()

	err := godotenv.Load("../../.env.test")
	require.NoError(t, err, "Failed to load .env.test file")

	cfg := &Config{}
	err = cfg.LoadFromEnv()
	require.NoError(t, err, "Failed to load config")
	return cfg
}

// TestLoadFromEnv tests loading configuration from environment variables
func TestLoadFromEnv(t *testing.T) {
	// Load test environment
	err := godotenv.Load("../../.env.test")
	require.NoError(t, err, "Failed to load .env.test file")

	cfg := &Config{}
	err = cfg.LoadFromEnv()
	require.NoError(t, err)

	// Verify configuration values
	require.Equal(t, "8080", cfg.API.Port)
	require.Equal(t, "localhost", cfg.Database.Host)
	require.Equal(t, 5432, cfg.Database.Port)
	require.Equal(t, "postgres", cfg.Database.User)
	require.Equal(t, "postgres", cfg.Database.Password)
	require.Equal(t, "wattwatch_test", cfg.Database.DBName)
	require.Equal(t, "disable", cfg.Database.SSLMode)
	require.Equal(t, "test_secret_key", cfg.Auth.JWTSecret)
	require.Equal(t, 24, cfg.Auth.JWTExpiration)
	require.True(t, cfg.Auth.RegistrationOpen)
}
