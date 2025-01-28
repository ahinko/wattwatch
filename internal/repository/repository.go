package repository

import (
	"context"
	"database/sql"
)

// Repository represents the base repository interface
type Repository interface {
	// Transaction executes operations within a database transaction
	Transaction(ctx context.Context, fn func(ctx context.Context) error) error
	DB() *sql.DB
}

// BaseRepository provides common functionality for all repositories
type BaseRepository struct {
	db *sql.DB
}

// NewBaseRepository creates a new base repository
func NewBaseRepository(db *sql.DB) BaseRepository {
	return BaseRepository{db: db}
}

// DB returns the database connection
func (r *BaseRepository) DB() *sql.DB {
	return r.db
}

// Transaction implements the Repository interface
func (r *BaseRepository) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	err = fn(ctx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return err
		}
		return err
	}

	return tx.Commit()
}
