package database

import (
	"context"
	"fmt"
	"time"

	"velyxora/packages/logger"
)

// Migration represents a single SQL migration script
type Migration struct {
	ID        int
	Name      string
	SQL       string
	Timestamp time.Time
}

// schemaMigrations lists all standard database tables to run on bootstrap
var schemaMigrations = []Migration{
	{
		ID:   1,
		Name: "create_users_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS users (
				id VARCHAR(255) PRIMARY KEY,
				email VARCHAR(255) UNIQUE NOT NULL,
				password_hash VARCHAR(255) NOT NULL,
				is_mfa_enabled BOOLEAN DEFAULT FALSE NOT NULL,
				mfa_secret VARCHAR(255) DEFAULT '' NOT NULL,
				status VARCHAR(50) NOT NULL,
				role VARCHAR(50) NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   2,
		Name: "create_balances_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS balances (
				user_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				available DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				locked DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				pending DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				reserved DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				total DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				PRIMARY KEY (user_id, asset)
			);
		`,
	},
	{
		ID:   3,
		Name: "create_ledger_entries_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS ledger_entries (
				id VARCHAR(255) PRIMARY KEY,
				ledger_tx_id VARCHAR(255) NOT NULL,
				user_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				type VARCHAR(50) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				description TEXT DEFAULT '' NOT NULL,
				timestamp TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   4,
		Name: "create_deposits_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS deposits (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				fee DECIMAL(36, 18) NOT NULL,
				address VARCHAR(255) NOT NULL,
				tx_hash VARCHAR(255) NOT NULL,
				confirmations INT DEFAULT 0 NOT NULL,
				status VARCHAR(50) NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   5,
		Name: "create_withdrawals_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS withdrawals (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				fee DECIMAL(36, 18) NOT NULL,
				address VARCHAR(255) NOT NULL,
				tx_hash VARCHAR(255) DEFAULT '' NOT NULL,
				status VARCHAR(50) NOT NULL,
				risk_score DECIMAL(5, 4) DEFAULT 0.0 NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
}

// RunMigrations executes schema migration steps on the pgx connection pool
func RunMigrations(ctx context.Context, db *DB) error {
	log := logger.NewLogger(logger.Config{
		Level:       "INFO",
		Format:      "JSON",
		ServiceName: "database-migration",
	})

	log.Info("Checking for database schema migrations...")

	// Create migrations history table
	createHistoryTableSQL := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id INT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
		);
	`
	_, err := db.Pool.Exec(ctx, createHistoryTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	for _, m := range schemaMigrations {
		var exists bool
		err = db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE id = $1)", m.ID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %d (%s): %w", m.ID, m.Name, err)
		}

		if !exists {
			log.Info(fmt.Sprintf("Running database migration %d: %s", m.ID, m.Name))

			// Run migration SQL
			_, err = db.Pool.Exec(ctx, m.SQL)
			if err != nil {
				return fmt.Errorf("failed to execute migration %d (%s): %w", m.ID, m.Name, err)
			}

			// Record history
			_, err = db.Pool.Exec(ctx, "INSERT INTO schema_migrations (id, name) VALUES ($1, $2)", m.ID, m.Name)
			if err != nil {
				return fmt.Errorf("failed to record migration history for %d (%s): %w", m.ID, m.Name, err)
			}

			log.Info(fmt.Sprintf("Migration %d (%s) executed successfully.", m.ID, m.Name))
		}
	}

	log.Info("All database schema migrations are up to date.")
	return nil
}
