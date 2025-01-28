// Package database handles database connections and migrations
package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"wattwatch/internal/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

// Connect establishes a connection to the database using the provided configuration
func Connect(cfg config.DatabaseConfig) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	return sql.Open("postgres", connStr)
}

// RunMigrations executes all pending database migrations
func RunMigrations(cfg config.DatabaseConfig) error {
	// Ensure we have an absolute path
	migrationsPath, err := filepath.Abs(cfg.MigrationsPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute migrations path: %w", err)
	}

	// Check if directory exists
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		return fmt.Errorf("migrations directory does not exist: %s", migrationsPath)
	}

	connectionString := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode,
	)

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		connectionString,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func SetupDatabase(cfg config.DatabaseConfig) (*sql.DB, error) {
	db, err := Connect(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Run migrations
	if err := RunMigrations(cfg); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}
