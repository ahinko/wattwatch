// Package db provides database utilities for testing
package db

import (
	"database/sql"
	"fmt"
	"testing"
	"wattwatch/internal/config"
	"wattwatch/internal/database"

	"github.com/stretchr/testify/require"
)

// CleanupTestDB drops all tables in the test database
func CleanupTestDB(db *sql.DB) error {
	// Get all table names
	rows, err := db.Query(`
		SELECT tablename 
		FROM pg_tables 
		WHERE schemaname = 'public'
		ORDER BY tablename
	`)
	if err != nil {
		return fmt.Errorf("failed to get table names: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating over table names: %w", err)
	}

	// Drop materialized views first
	_, err = db.Exec(`
		DROP MATERIALIZED VIEW IF EXISTS spot_prices_monthly CASCADE;
		DROP MATERIALIZED VIEW IF EXISTS spot_prices_daily CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("failed to drop materialized views: %w", err)
	}

	// Drop each table individually
	for _, table := range tables {
		dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)
		if _, err := db.Exec(dropQuery); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	return nil
}

func SetupTestDB(t *testing.T, cfg *config.DatabaseConfig) *sql.DB {
	t.Helper()

	db, err := database.Connect(*cfg)
	require.NoError(t, err, "Failed to connect to test database")

	// Clean up any existing tables
	err = CleanupTestDB(db)
	require.NoError(t, err, "Failed to cleanup test database")

	// Verify tables are dropped
	var tableCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM pg_tables WHERE schemaname = 'public'`).Scan(&tableCount)
	require.NoError(t, err, "Failed to count tables")
	require.Equal(t, 0, tableCount, "Database should be empty before running migrations")

	// Run migrations using the same setup as the main app
	err = database.RunMigrations(*cfg)
	require.NoError(t, err, "Failed to run migrations")

	return db
}
