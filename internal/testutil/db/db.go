// Package db provides database utilities for testing
package db

import (
	"database/sql"
	"fmt"
	"strings"
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

	if len(tables) > 0 {
		// Start a transaction for dropping tables
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Drop all tables
		dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE",
			strings.Join(tables, ", "))
		_, err = tx.Exec(dropQuery)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to drop tables: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
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
