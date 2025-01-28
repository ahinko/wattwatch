package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
	"wattwatch/internal/provider"

	_ "github.com/lib/pq"
)

// Config represents the application configuration
type Config struct {
	// API contains API server configuration
	API APIConfig
	// Auth contains authentication configuration
	Auth AuthConfig
	// Database contains database configuration
	Database DatabaseConfig
	// Email contains email service configuration
	Email EmailConfig
	// JWT settings
	JWTSecret            string        `envconfig:"JWT_SECRET" required:"true"`
	AccessTokenDuration  time.Duration `envconfig:"ACCESS_TOKEN_DURATION" default:"15m"`
	RefreshTokenDuration time.Duration `envconfig:"REFRESH_TOKEN_DURATION" default:"168h"` // 7 days

	// Rate Limiting Configuration
	RateLimit struct {
		Requests int `envconfig:"RATE_LIMIT_REQUESTS" default:"1000"` // Number of requests allowed per window
		Window   int `envconfig:"RATE_LIMIT_WINDOW" default:"60"`     // Time window in seconds
		Burst    int `envconfig:"RATE_LIMIT_BURST" default:"50"`      // Maximum burst size
	}

	DB       *sql.DB                    `json:"-"` // Connection pool, not serialized
	Provider map[string]provider.Config `json:"providers"`
}

// DatabaseConfig contains database connection settings
type DatabaseConfig struct {
	// Host is the database server hostname
	Host string
	// Port is the database server port
	Port int
	// User is the database username
	User string
	// Password is the database password
	Password string
	// DBName is the database name
	DBName string
	// SSLMode is the SSL mode for the database connection
	SSLMode string
	// MigrationsPath is the path to database migrations
	MigrationsPath string
}

// APIConfig contains API server settings
type APIConfig struct {
	// Port is the server port to listen on
	Port string
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	// JWTSecret is the secret key used to sign JWT tokens
	JWTSecret string
	// JWTExpiration is the JWT token expiration time in hours
	JWTExpiration int
	// RegistrationOpen determines if new user registration is allowed
	RegistrationOpen bool
}

// EmailConfig contains email service settings
type EmailConfig struct {
	// SMTPHost is the SMTP server hostname
	SMTPHost string
	// SMTPPort is the SMTP server port
	SMTPPort int
	// SMTPUsername is the SMTP authentication username
	SMTPUsername string
	// SMTPPassword is the SMTP authentication password
	SMTPPassword string
	// FromAddress is the email address used as sender
	FromAddress string
	// AppURL is the base URL of the application
	AppURL string
}

// ProviderConfig represents configuration for a data provider
type ProviderConfig struct {
	Enabled bool `json:"enabled"`
	// Add other provider-specific fields as needed
}

// Load reads the configuration from a file and establishes database connection
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Create database connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)

	// Open database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	cfg.DB = db
	return &cfg, nil
}

// Close releases any resources held by the configuration
func (c *Config) Close() error {
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}

// LoadFromEnv retrieves configuration from environment variables
func (c *Config) LoadFromEnv() error {
	c.API = APIConfig{
		Port: getEnvOrDefault("API_PORT", "8080"),
	}
	c.Database = DatabaseConfig{
		Host:           getEnvOrDefault("DB_HOST", "localhost"),
		Port:           getEnvAsInt("DB_PORT", 5432),
		User:           getEnvOrDefault("DB_USER", "postgres"),
		Password:       getEnvOrDefault("DB_PASSWORD", "postgres"),
		DBName:         getEnvOrDefault("DB_NAME", "wattwatch"),
		SSLMode:        getEnvOrDefault("DB_SSL_MODE", "disable"),
		MigrationsPath: "migrations",
	}
	c.Auth = AuthConfig{
		JWTSecret:        os.Getenv("JWT_SECRET"),
		JWTExpiration:    getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
		RegistrationOpen: getEnvAsBool("REGISTRATION_OPEN", true),
	}
	c.Email = EmailConfig{
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     getEnvAsInt("SMTP_PORT", 587),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		FromAddress:  os.Getenv("SMTP_FROM"),
		AppURL:       os.Getenv("APP_URL"),
	}

	// Initialize provider configuration
	c.Provider = make(map[string]provider.Config)
	c.Provider["nordpool"] = provider.Config{
		Enabled: getEnvAsBool("ENABLE_NORDPOOL", false),
	}

	// Load rate limit configuration
	c.RateLimit.Requests = getEnvAsInt("RATE_LIMIT_REQUESTS", 1000)
	c.RateLimit.Window = getEnvAsInt("RATE_LIMIT_WINDOW", 60)
	c.RateLimit.Burst = getEnvAsInt("RATE_LIMIT_BURST", 50)

	// Validate required fields
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}

	return nil
}

// getEnvAsInt retrieves an environment variable and converts it to an integer
func getEnvAsInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

// getEnvAsBool retrieves an environment variable and converts it to a boolean
func getEnvAsBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}

func getEnvOrDefault(key string, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
