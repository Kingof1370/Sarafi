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
	{
		ID:   6,
		Name: "create_orders_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS orders (
				id VARCHAR(255) PRIMARY KEY,
				client_order_id VARCHAR(255) DEFAULT '' NOT NULL,
				external_reference_id VARCHAR(255) DEFAULT '' NOT NULL,
				execution_id VARCHAR(255) DEFAULT '' NOT NULL,
				correlation_id VARCHAR(255) DEFAULT '' NOT NULL,
				user_id VARCHAR(255) NOT NULL,
				symbol VARCHAR(100) NOT NULL,
				side VARCHAR(50) NOT NULL,
				type VARCHAR(50) NOT NULL,
				price DECIMAL(36, 18) NOT NULL,
				quantity DECIMAL(36, 18) NOT NULL,
				filled_quantity DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				status VARCHAR(50) NOT NULL,
				time_in_force VARCHAR(50) DEFAULT 'GTC' NOT NULL,
				expire_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				stop_price DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				trailing_delta DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				iceberg_size DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				post_only BOOLEAN DEFAULT FALSE NOT NULL,
				reduce_only BOOLEAN DEFAULT FALSE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   7,
		Name: "create_order_history_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS order_history (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				symbol VARCHAR(100) NOT NULL,
				side VARCHAR(50) NOT NULL,
				type VARCHAR(50) NOT NULL,
				price DECIMAL(36, 18) NOT NULL,
				quantity DECIMAL(36, 18) NOT NULL,
				filled_quantity DECIMAL(36, 18) NOT NULL,
				status VARCHAR(50) NOT NULL,
				created_at TIMESTAMP NOT NULL,
				updated_at TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   8,
		Name: "create_order_events_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS order_events (
				id VARCHAR(255) PRIMARY KEY,
				order_id VARCHAR(255) NOT NULL,
				event_type VARCHAR(50) NOT NULL,
				payload TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   9,
		Name: "create_order_audits_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS order_audits (
				id VARCHAR(255) PRIMARY KEY,
				order_id VARCHAR(255) NOT NULL,
				user_id VARCHAR(255) NOT NULL,
				action VARCHAR(100) NOT NULL,
				previous_state VARCHAR(50) NOT NULL,
				new_state VARCHAR(50) NOT NULL,
				ip_address VARCHAR(100) NOT NULL,
				device_id VARCHAR(100) NOT NULL,
				result TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   10,
		Name: "create_risk_rules_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS risk_rules (
				id VARCHAR(255) PRIMARY KEY,
				symbol VARCHAR(100) NOT NULL,
				max_order_size DECIMAL(36, 18) NOT NULL,
				max_daily_volume DECIMAL(36, 18) NOT NULL,
				max_open_orders INT NOT NULL,
				price_band_percentage DECIMAL(5, 4) NOT NULL,
				is_active BOOLEAN DEFAULT TRUE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   11,
		Name: "create_risk_events_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS risk_events (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				event_type VARCHAR(100) NOT NULL,
				details TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   12,
		Name: "create_risk_violations_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS risk_violations (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				violation_type VARCHAR(100) NOT NULL,
				ip_address VARCHAR(100) NOT NULL,
				details TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   13,
		Name: "create_positions_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS positions (
				user_id VARCHAR(255) NOT NULL,
				symbol VARCHAR(100) NOT NULL,
				size DECIMAL(36, 18) NOT NULL,
				entry_price DECIMAL(36, 18) NOT NULL,
				realized_pnl DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				unrealized_pnl DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				PRIMARY KEY (user_id, symbol)
			);
		`,
	},
	{
		ID:   14,
		Name: "create_settlements_queue_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS settlements_queue (
				id VARCHAR(255) PRIMARY KEY,
				trade_id VARCHAR(255) NOT NULL,
				status VARCHAR(50) NOT NULL,
				retries INT DEFAULT 0 NOT NULL,
				max_retries INT DEFAULT 3 NOT NULL,
				error_msg TEXT DEFAULT '' NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   15,
		Name: "create_settlements_history_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS settlements_history (
				id VARCHAR(255) PRIMARY KEY,
				trade_id VARCHAR(255) NOT NULL,
				buyer_id VARCHAR(255) NOT NULL,
				seller_id VARCHAR(255) NOT NULL,
				price DECIMAL(36, 18) NOT NULL,
				quantity DECIMAL(36, 18) NOT NULL,
				status VARCHAR(50) NOT NULL,
				completed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   16,
		Name: "create_fee_ledgers_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS fee_ledgers (
				id VARCHAR(255) PRIMARY KEY,
				trade_id VARCHAR(255) NOT NULL,
				user_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				role VARCHAR(50) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   17,
		Name: "create_accounting_reconciliations_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS accounting_reconciliations (
				id VARCHAR(255) PRIMARY KEY,
				batch_id VARCHAR(255) NOT NULL,
				total_debit DECIMAL(36, 18) NOT NULL,
				total_credit DECIMAL(36, 18) NOT NULL,
				total_fee DECIMAL(36, 18) NOT NULL,
				is_reconciled BOOLEAN DEFAULT TRUE NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   18,
		Name: "create_liquidity_metrics_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS liquidity_metrics (
				id VARCHAR(255) PRIMARY KEY,
				symbol VARCHAR(100) NOT NULL,
				spread DECIMAL(36, 18) NOT NULL,
				mid_price DECIMAL(36, 18) NOT NULL,
				weighted_mid_price DECIMAL(36, 18) NOT NULL,
				depth_imbalance DECIMAL(10, 8) NOT NULL,
				market_health_score DECIMAL(5, 2) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   19,
		Name: "create_spread_history_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS spread_history (
				id VARCHAR(255) PRIMARY KEY,
				symbol VARCHAR(100) NOT NULL,
				bid DECIMAL(36, 18) NOT NULL,
				ask DECIMAL(36, 18) NOT NULL,
				spread DECIMAL(36, 18) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   20,
		Name: "create_depth_history_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS depth_history (
				id VARCHAR(255) PRIMARY KEY,
				symbol VARCHAR(100) NOT NULL,
				bid_volume DECIMAL(36, 18) NOT NULL,
				ask_volume DECIMAL(36, 18) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   21,
		Name: "create_surveillance_alerts_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS surveillance_alerts (
				id VARCHAR(255) PRIMARY KEY,
				symbol VARCHAR(100) NOT NULL,
				user_id VARCHAR(255) NOT NULL,
				pattern VARCHAR(100) NOT NULL,
				details TEXT NOT NULL,
				severity VARCHAR(50) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   22,
		Name: "create_fee_rules_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS fee_rules (
				id VARCHAR(255) PRIMARY KEY,
				symbol VARCHAR(100) NOT NULL,
				maker_rate DECIMAL(10, 8) NOT NULL,
				taker_rate DECIMAL(10, 8) NOT NULL,
				is_active BOOLEAN DEFAULT TRUE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   23,
		Name: "create_fee_history_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS fee_history (
				id VARCHAR(255) PRIMARY KEY,
				trade_id VARCHAR(255) NOT NULL,
				user_id VARCHAR(255) NOT NULL,
				fee_type VARCHAR(50) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   24,
		Name: "create_vip_levels_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS vip_levels (
				level INT PRIMARY KEY,
				min_volume DECIMAL(36, 18) NOT NULL,
				maker_rate DECIMAL(10, 8) NOT NULL,
				taker_rate DECIMAL(10, 8) NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   25,
		Name: "create_referral_commissions_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS referral_commissions (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				referrer_id VARCHAR(255) NOT NULL,
				trade_id VARCHAR(255) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   26,
		Name: "create_monitoring_metrics_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS monitoring_metrics (
				id VARCHAR(255) PRIMARY KEY,
				cpu_usage DECIMAL(5, 2) NOT NULL,
				memory_usage BIGINT NOT NULL,
				goroutines INT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   27,
		Name: "create_health_checks_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS health_checks (
				id VARCHAR(255) PRIMARY KEY,
				service_name VARCHAR(100) NOT NULL,
				status VARCHAR(50) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   28,
		Name: "create_backup_history_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS backup_history (
				id VARCHAR(255) PRIMARY KEY,
				backup_type VARCHAR(50) NOT NULL,
				status VARCHAR(50) NOT NULL,
				filepath VARCHAR(255) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   29,
		Name: "create_recovery_history_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS recovery_history (
				id VARCHAR(255) PRIMARY KEY,
				replayed_count INT NOT NULL,
				status VARCHAR(50) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   30,
		Name: "create_permissions_and_rbac_tables",
		SQL: `
			CREATE TABLE IF NOT EXISTS permissions (
				name VARCHAR(100) PRIMARY KEY,
				description TEXT NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);

			CREATE TABLE IF NOT EXISTS role_permissions (
				role VARCHAR(100) NOT NULL,
				permission VARCHAR(100) NOT NULL,
				PRIMARY KEY (role, permission),
				FOREIGN KEY (permission) REFERENCES permissions(name) ON DELETE CASCADE
			);

			-- Seed default permissions
			INSERT INTO permissions (name, description) VALUES
				('wallet:read', 'Read wallet balance and addresses'),
				('wallet:write', 'Withdraw assets and request new addresses'),
				('trading:write', 'Place and cancel orders'),
				('support:read', 'Support agent views and access'),
				('risk:write', 'Compliance controls, risk halt, and user block'),
				('system:admin', 'System operations like backup and recovery'),
				('super:admin', 'Global destructive actions')
			ON CONFLICT (name) DO NOTHING;

			-- Seed default role-to-permission mappings
			INSERT INTO role_permissions (role, permission) VALUES
				('USER', 'wallet:read'),
				('USER', 'wallet:write'),
				('USER', 'trading:write'),

				('VERIFIED_USER', 'wallet:read'),
				('VERIFIED_USER', 'wallet:write'),
				('VERIFIED_USER', 'trading:write'),

				('VIP_USER', 'wallet:read'),
				('VIP_USER', 'wallet:write'),
				('VIP_USER', 'trading:write'),

				('SUPPORT_AGENT', 'wallet:read'),
				('SUPPORT_AGENT', 'support:read'),

				('COMPLIANCE_OFFICER', 'wallet:read'),
				('COMPLIANCE_OFFICER', 'support:read'),
				('COMPLIANCE_OFFICER', 'risk:write'),

				('SECURITY_OFFICER', 'wallet:read'),
				('SECURITY_OFFICER', 'support:read'),
				('SECURITY_OFFICER', 'risk:write'),

				('ADMIN', 'wallet:read'),
				('ADMIN', 'wallet:write'),
				('ADMIN', 'trading:write'),
				('ADMIN', 'support:read'),
				('ADMIN', 'risk:write'),
				('ADMIN', 'system:admin'),

				('SUPER_ADMIN', 'wallet:read'),
				('SUPER_ADMIN', 'wallet:write'),
				('SUPER_ADMIN', 'trading:write'),
				('SUPER_ADMIN', 'support:read'),
				('SUPER_ADMIN', 'risk:write'),
				('SUPER_ADMIN', 'system:admin'),
				('SUPER_ADMIN', 'super:admin')
			ON CONFLICT (role, permission) DO NOTHING;
		`,
	},
	{
		ID:   31,
		Name: "create_sessions_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS user_sessions (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				device_id VARCHAR(255) DEFAULT '' NOT NULL,
				ip_address VARCHAR(100) DEFAULT '' NOT NULL,
				user_agent VARCHAR(255) DEFAULT '' NOT NULL,
				refresh_token_hash VARCHAR(255) NOT NULL,
				expires_at TIMESTAMP NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				is_revoked BOOLEAN DEFAULT FALSE NOT NULL,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_sessions_user ON user_sessions(user_id);
			CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON user_sessions(refresh_token_hash);
		`,
	},
	{
		ID:   32,
		Name: "create_api_keys_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS user_api_keys (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				label VARCHAR(255) DEFAULT '' NOT NULL,
				api_key VARCHAR(255) UNIQUE NOT NULL,
				api_secret_hash VARCHAR(255) NOT NULL,
				permissions TEXT DEFAULT '' NOT NULL,
				ip_allowlist TEXT DEFAULT '' NOT NULL,
				expires_at TIMESTAMP NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				is_revoked BOOLEAN DEFAULT FALSE NOT NULL,
				last_used_at TIMESTAMP,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_api_keys_user ON user_api_keys(user_id);
			CREATE INDEX IF NOT EXISTS idx_api_keys_key ON user_api_keys(api_key);
		`,
	},
	{
		ID:   33,
		Name: "create_brute_force_lockouts_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS brute_force_lockouts (
				id VARCHAR(255) PRIMARY KEY,
				identity_key VARCHAR(255) UNIQUE NOT NULL,
				failed_attempts INT DEFAULT 0 NOT NULL,
				locked_until TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   34,
		Name: "create_security_audit_logs_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS security_audit_logs (
				id VARCHAR(255) PRIMARY KEY,
				event_type VARCHAR(100) NOT NULL,
				severity VARCHAR(50) NOT NULL,
				user_id VARCHAR(255) DEFAULT '' NOT NULL,
				api_key_id VARCHAR(255) DEFAULT '' NOT NULL,
				session_id VARCHAR(255) DEFAULT '' NOT NULL,
				ip_address VARCHAR(100) DEFAULT '' NOT NULL,
				user_agent VARCHAR(255) DEFAULT '' NOT NULL,
				details TEXT DEFAULT '' NOT NULL,
				metadata TEXT DEFAULT '' NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_audit_logs_user ON security_audit_logs(user_id);
			CREATE INDEX IF NOT EXISTS idx_audit_logs_type ON security_audit_logs(event_type);
		`,
	},
	{
		ID:   35,
		Name: "create_idempotency_records_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS idempotency_records (
				id_key VARCHAR(255) NOT NULL,
				user_id VARCHAR(255) NOT NULL,
				operation VARCHAR(255) NOT NULL,
				request_payload_hash VARCHAR(255) NOT NULL,
				status VARCHAR(50) NOT NULL,
				response_status INT NOT NULL,
				response_body TEXT NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				PRIMARY KEY (id_key, user_id)
			);
			CREATE INDEX IF NOT EXISTS idx_idempotency_user ON idempotency_records(user_id);
		`,
	},
	{
		ID:   36,
		Name: "create_p0008_tables",
		SQL: `
			CREATE TABLE IF NOT EXISTS wallet_addresses (
				user_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				address VARCHAR(255) PRIMARY KEY,
				memo VARCHAR(255) DEFAULT '' NOT NULL,
				is_active BOOLEAN DEFAULT TRUE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_wallet_addresses_user ON wallet_addresses(user_id);

			CREATE TABLE IF NOT EXISTS custody_vaults (
				id VARCHAR(255) PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				role VARCHAR(50) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				address VARCHAR(255) NOT NULL,
				vault_limit DECIMAL(36, 18) NOT NULL,
				balance DECIMAL(36, 18) NOT NULL,
				is_frozen BOOLEAN DEFAULT FALSE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);

			CREATE TABLE IF NOT EXISTS custody_approvals (
				id VARCHAR(255) PRIMARY KEY,
				withdrawal_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				status VARCHAR(50) NOT NULL,
				first_approver VARCHAR(255) DEFAULT '' NOT NULL,
				second_approver VARCHAR(255) DEFAULT '' NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);

			CREATE TABLE IF NOT EXISTS custody_emergency_logs (
				id VARCHAR(255) PRIMARY KEY,
				action VARCHAR(100) NOT NULL,
				initiated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);

			CREATE TABLE IF NOT EXISTS blockchain_scanner_states (
				network VARCHAR(50) PRIMARY KEY,
				last_scanned_height INT NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);

			CREATE TABLE IF NOT EXISTS reconciliation_runs (
				id VARCHAR(255) PRIMARY KEY,
				status VARCHAR(50) NOT NULL,
				started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				completed_at TIMESTAMP,
				details TEXT
			);

			CREATE TABLE IF NOT EXISTS reconciliation_issues (
				id VARCHAR(255) PRIMARY KEY,
				run_id VARCHAR(255) NOT NULL,
				layer VARCHAR(50) NOT NULL,
				severity VARCHAR(50) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				details TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
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
