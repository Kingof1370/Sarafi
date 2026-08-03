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
		Name: "create_blockchain_networks_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS blockchain_networks (
				id VARCHAR(100) PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				derivation_path VARCHAR(255) NOT NULL,
				is_active BOOLEAN DEFAULT TRUE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_blockchain_networks_active ON blockchain_networks (is_active);
		`,
	},
	{
		ID:   31,
		Name: "create_wallets_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS wallets (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				type VARCHAR(100) NOT NULL,
				is_locked BOOLEAN DEFAULT FALSE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets (user_id);
			CREATE INDEX IF NOT EXISTS idx_wallets_type ON wallets (type);
		`,
	},
	{
		ID:   32,
		Name: "create_assets_registry_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS assets_registry (
				symbol VARCHAR(50) PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				type VARCHAR(50) NOT NULL,
				precision INT DEFAULT 8 NOT NULL,
				base_network VARCHAR(100) NOT NULL,
				is_active BOOLEAN DEFAULT TRUE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_assets_registry_active ON assets_registry (is_active);
		`,
	},
	{
		ID:   33,
		Name: "create_assets_metadata_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS assets_metadata (
				symbol VARCHAR(50) PRIMARY KEY,
				contract_address VARCHAR(255) DEFAULT '' NOT NULL,
				logo_url VARCHAR(255) DEFAULT '' NOT NULL,
				description TEXT DEFAULT '' NOT NULL,
				website VARCHAR(255) DEFAULT '' NOT NULL,
				explorer_url VARCHAR(255) DEFAULT '' NOT NULL,
				total_supply VARCHAR(255) DEFAULT '' NOT NULL,
				circulating_price DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				FOREIGN KEY (symbol) REFERENCES assets_registry (symbol) ON DELETE CASCADE
			);
		`,
	},
	{
		ID:   34,
		Name: "create_assets_permissions_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS assets_permissions (
				symbol VARCHAR(50) PRIMARY KEY,
				can_deposit BOOLEAN DEFAULT TRUE NOT NULL,
				can_withdraw BOOLEAN DEFAULT TRUE NOT NULL,
				can_trade BOOLEAN DEFAULT TRUE NOT NULL,
				min_deposit_amount DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				min_withdraw_amount DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				max_daily_withdrawal DECIMAL(36, 18) DEFAULT 1000.0 NOT NULL,
				withdrawal_fee DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				FOREIGN KEY (symbol) REFERENCES assets_registry (symbol) ON DELETE CASCADE
			);
		`,
	},
	{
		ID:   35,
		Name: "create_wallet_balances_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS wallet_balances (
				wallet_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				available DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				locked DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				reserved DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				pending DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				total DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				PRIMARY KEY (wallet_id, asset),
				FOREIGN KEY (wallet_id) REFERENCES wallets (id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_wallet_balances_asset ON wallet_balances (asset);
		`,
	},
	{
		ID:   36,
		Name: "create_wallet_balance_history_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS wallet_balance_history (
				id VARCHAR(255) PRIMARY KEY,
				wallet_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				available DECIMAL(36, 18) NOT NULL,
				locked DECIMAL(36, 18) NOT NULL,
				reserved DECIMAL(36, 18) NOT NULL,
				pending DECIMAL(36, 18) NOT NULL,
				total DECIMAL(36, 18) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				FOREIGN KEY (wallet_id) REFERENCES wallets (id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_balance_history_wallet ON wallet_balance_history (wallet_id);
			CREATE INDEX IF NOT EXISTS idx_balance_history_asset ON wallet_balance_history (asset);
		`,
	},
	{
		ID:   37,
		Name: "create_wallet_addresses_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS wallet_addresses (
				address VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				network VARCHAR(100) NOT NULL,
				public_key BYTEA NOT NULL,
				derivation_path VARCHAR(255) NOT NULL,
				memo VARCHAR(255) DEFAULT '' NOT NULL,
				status VARCHAR(50) NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_wallet_addresses_user_network ON wallet_addresses (user_id, network);
		`,
	},
	{
		ID:   38,
		Name: "create_wallet_events_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS wallet_events (
				id VARCHAR(255) PRIMARY KEY,
				event_type VARCHAR(100) NOT NULL,
				wallet_id VARCHAR(255) NOT NULL,
				payload TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_wallet_events_type_wallet ON wallet_events (event_type, wallet_id);
		`,
	},
	{
		ID:   39,
		Name: "create_wallet_audits_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS wallet_audits (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				wallet_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				action VARCHAR(100) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				prev_balance DECIMAL(36, 18) NOT NULL,
				new_balance DECIMAL(36, 18) NOT NULL,
				message TEXT NOT NULL,
				prev_hash VARCHAR(255) NOT NULL,
				hash VARCHAR(255) NOT NULL,
				timestamp TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_wallet_audits_user_asset ON wallet_audits (user_id, asset);
			CREATE INDEX IF NOT EXISTS idx_wallet_audits_wallet ON wallet_audits (wallet_id);
		`,
	},
	{
		ID:   40,
		Name: "create_blockchain_transactions_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS blockchain_transactions (
				tx_hash VARCHAR(255) PRIMARY KEY,
				network VARCHAR(100) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				sender VARCHAR(255) NOT NULL,
				receiver VARCHAR(255) NOT NULL,
				block_number BIGINT NOT NULL,
				gas_used BIGINT DEFAULT 0 NOT NULL,
				timestamp TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_blockchain_tx_network_block ON blockchain_transactions (network, block_number);
		`,
	},
	{
		ID:   41,
		Name: "create_deposit_confirmations_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS deposit_confirmations (
				deposit_id VARCHAR(255) PRIMARY KEY,
				confirmations_count INT NOT NULL,
				required_confirmations INT NOT NULL,
				status VARCHAR(50) NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_deposit_conf_status ON deposit_confirmations (status);
		`,
	},
	{
		ID:   42,
		Name: "create_deposit_events_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS deposit_events (
				id VARCHAR(255) PRIMARY KEY,
				event_type VARCHAR(100) NOT NULL,
				deposit_id VARCHAR(255) NOT NULL,
				payload TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_deposit_events_type_dep ON deposit_events (event_type, deposit_id);
		`,
	},
	{
		ID:   43,
		Name: "create_withdrawal_address_book_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS withdrawal_address_book (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				address VARCHAR(255) NOT NULL,
				network VARCHAR(100) NOT NULL,
				label VARCHAR(255) DEFAULT '' NOT NULL,
				is_whitelisted BOOLEAN DEFAULT TRUE NOT NULL,
				trust_score DECIMAL(5, 4) DEFAULT 1.0 NOT NULL,
				verified_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				UNIQUE (user_id, address)
			);
			CREATE INDEX IF NOT EXISTS idx_withdraw_addr_user_addr ON withdrawal_address_book (user_id, address);
		`,
	},
	{
		ID:   44,
		Name: "create_withdrawal_queue_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS withdrawal_queue (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				fee DECIMAL(36, 18) NOT NULL,
				address VARCHAR(255) NOT NULL,
				status VARCHAR(50) NOT NULL,
				risk_score DECIMAL(5, 4) DEFAULT 0.0 NOT NULL,
				retries INT DEFAULT 0 NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_withdrawal_q_status ON withdrawal_queue (status);
			CREATE INDEX IF NOT EXISTS idx_withdrawal_q_user ON withdrawal_queue (user_id);
		`,
	},
	{
		ID:   45,
		Name: "create_withdrawal_approvals_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS withdrawal_approvals (
				id VARCHAR(255) PRIMARY KEY,
				withdrawal_id VARCHAR(255) NOT NULL,
				admin_id VARCHAR(255) NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				FOREIGN KEY (withdrawal_id) REFERENCES withdrawal_queue (id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_withdrawal_app_id ON withdrawal_approvals (withdrawal_id);
		`,
	},
	{
		ID:   46,
		Name: "create_withdrawal_audits_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS withdrawal_audits (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				withdrawal_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				prev_status VARCHAR(50) NOT NULL,
				new_status VARCHAR(50) NOT NULL,
				message TEXT NOT NULL,
				timestamp TIMESTAMP NOT NULL,
				FOREIGN KEY (withdrawal_id) REFERENCES withdrawal_queue (id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_withdrawal_aud_user ON withdrawal_audits (user_id);
			CREATE INDEX IF NOT EXISTS idx_withdrawal_aud_id ON withdrawal_audits (withdrawal_id);
		`,
	},
	{
		ID:   47,
		Name: "create_withdrawal_risk_events_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS withdrawal_risk_events (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				withdrawal_id VARCHAR(255) NOT NULL,
				details TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				FOREIGN KEY (withdrawal_id) REFERENCES withdrawal_queue (id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_withdrawal_risk_id ON withdrawal_risk_events (withdrawal_id);
		`,
	},
	{
		ID:   48,
		Name: "create_withdrawal_broadcast_history_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS withdrawal_broadcast_history (
				tx_hash VARCHAR(255) PRIMARY KEY,
				withdrawal_id VARCHAR(255) NOT NULL,
				payload_hex TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				FOREIGN KEY (withdrawal_id) REFERENCES withdrawal_queue (id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_withdrawal_broad_id ON withdrawal_broadcast_history (withdrawal_id);
		`,
	},
	{
		ID:   49,
		Name: "create_blockchain_nodes_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS blockchain_nodes (
				id VARCHAR(255) PRIMARY KEY,
				network VARCHAR(100) NOT NULL,
				url VARCHAR(255) NOT NULL,
				type VARCHAR(50) NOT NULL,
				is_active BOOLEAN DEFAULT TRUE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_blockchain_nodes_net ON blockchain_nodes (network);
		`,
	},
	{
		ID:   50,
		Name: "create_rpc_endpoints_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS rpc_endpoints (
				id VARCHAR(255) PRIMARY KEY,
				node_id VARCHAR(255) NOT NULL,
				method_name VARCHAR(100) NOT NULL,
				is_supported BOOLEAN DEFAULT TRUE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				FOREIGN KEY (node_id) REFERENCES blockchain_nodes (id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_rpc_endpoints_node ON rpc_endpoints (node_id);
		`,
	},
	{
		ID:   51,
		Name: "create_network_status_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS network_status (
				network VARCHAR(100) PRIMARY KEY,
				latest_block BIGINT DEFAULT 0 NOT NULL,
				status VARCHAR(50) NOT NULL,
				latency_ms BIGINT DEFAULT 0 NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   52,
		Name: "create_blockchain_sync_history_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS blockchain_sync_history (
				id VARCHAR(255) PRIMARY KEY,
				network VARCHAR(100) NOT NULL,
				block_height BIGINT NOT NULL,
				block_hash VARCHAR(255) NOT NULL,
				status VARCHAR(50) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_blockchain_sync_net ON blockchain_sync_history (network);
		`,
	},
	{
		ID:   53,
		Name: "create_node_metrics_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS node_metrics (
				id VARCHAR(255) PRIMARY KEY,
				node_id VARCHAR(255) NOT NULL,
				latency_ms BIGINT NOT NULL,
				failed_requests INT NOT NULL,
				health_score DECIMAL(5, 4) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				FOREIGN KEY (node_id) REFERENCES blockchain_nodes (id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_node_metrics_node ON node_metrics (node_id);
		`,
	},
	{
		ID:   54,
		Name: "create_cryptographic_keys_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS cryptographic_keys (
				id VARCHAR(255) PRIMARY KEY,
				version INT NOT NULL,
				type VARCHAR(50) NOT NULL,
				public_key BYTEA NOT NULL,
				fingerprint VARCHAR(255) NOT NULL,
				status VARCHAR(50) NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				expiration_date TIMESTAMP NOT NULL,
				is_hsm_managed BOOLEAN DEFAULT FALSE NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_crypto_keys_status ON cryptographic_keys (status);
			CREATE INDEX IF NOT EXISTS idx_crypto_keys_fingerprint ON cryptographic_keys (fingerprint);
		`,
	},
	{
		ID:   55,
		Name: "create_key_rotation_history_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS key_rotation_history (
				id VARCHAR(255) PRIMARY KEY,
				old_key_id VARCHAR(255) NOT NULL,
				new_key_id VARCHAR(255) NOT NULL,
				rotated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_key_rotation_old ON key_rotation_history (old_key_id);
			CREATE INDEX IF NOT EXISTS idx_key_rotation_new ON key_rotation_history (new_key_id);
		`,
	},
	{
		ID:   56,
		Name: "create_key_audit_logs_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS key_audit_logs (
				id VARCHAR(255) PRIMARY KEY,
				key_id VARCHAR(255) NOT NULL,
				action VARCHAR(100) NOT NULL,
				message TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_key_audit_logs_id ON key_audit_logs (key_id);
		`,
	},
	{
		ID:   57,
		Name: "create_signature_requests_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS signature_requests (
				id VARCHAR(255) PRIMARY KEY,
				raw_data BYTEA NOT NULL,
				required_approvals INT NOT NULL,
				current_approvals INT NOT NULL,
				status VARCHAR(50) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_sig_requests_status ON signature_requests (status);
		`,
	},
	{
		ID:   58,
		Name: "create_signature_approvals_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS signature_approvals (
				id VARCHAR(255) PRIMARY KEY,
				signature_request_id VARCHAR(255) NOT NULL,
				admin_id VARCHAR(255) NOT NULL,
				signature BYTEA NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				FOREIGN KEY (signature_request_id) REFERENCES signature_requests (id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_sig_approvals_req ON signature_approvals (signature_request_id);
		`,
	},
	{
		ID:   59,
		Name: "create_custody_vaults_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS custody_vaults (
				id VARCHAR(255) PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				type VARCHAR(50) NOT NULL,
				is_locked BOOLEAN DEFAULT FALSE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   60,
		Name: "create_vault_assets_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS vault_assets (
				vault_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				balance DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				PRIMARY KEY (vault_id, asset),
				FOREIGN KEY (vault_id) REFERENCES custody_vaults (id) ON DELETE CASCADE
			);
		`,
	},
	{
		ID:   61,
		Name: "create_custody_transfers_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS custody_transfers (
				id VARCHAR(255) PRIMARY KEY,
				from_vault_id VARCHAR(255) NOT NULL,
				to_vault_id VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				status VARCHAR(50) NOT NULL,
				required_approvals INT NOT NULL,
				current_approvals INT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				FOREIGN KEY (from_vault_id) REFERENCES custody_vaults (id) ON DELETE CASCADE,
				FOREIGN KEY (to_vault_id) REFERENCES custody_vaults (id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_custody_trans_status ON custody_transfers (status);
		`,
	},
	{
		ID:   62,
		Name: "create_custody_transfer_approvals_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS custody_transfer_approvals (
				id VARCHAR(255) PRIMARY KEY,
				transfer_id VARCHAR(255) NOT NULL,
				admin_id VARCHAR(255) NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				FOREIGN KEY (transfer_id) REFERENCES custody_transfers (id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_custody_app_trans ON custody_transfer_approvals (transfer_id);
		`,
	},
	{
		ID:   63,
		Name: "create_custody_audits_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS custody_audits (
				id VARCHAR(255) PRIMARY KEY,
				vault_id VARCHAR(255),
				transfer_id VARCHAR(255),
				asset VARCHAR(50) NOT NULL,
				action VARCHAR(100) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				message TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_custody_audits_vault ON custody_audits (vault_id);
		`,
	},
	{
		ID:   64,
		Name: "create_custody_emergency_actions_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS custody_emergency_actions (
				id VARCHAR(255) PRIMARY KEY,
				action VARCHAR(50) NOT NULL,
				operator_id VARCHAR(255) NOT NULL,
				details TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   65,
		Name: "create_treasury_pools_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS treasury_pools (
				id VARCHAR(100) PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				type VARCHAR(100) NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE TABLE IF NOT EXISTS pool_assets (
				pool_id VARCHAR(100) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				balance DECIMAL(36, 18) DEFAULT 0.0 NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				PRIMARY KEY (pool_id, asset),
				FOREIGN KEY (pool_id) REFERENCES treasury_pools (id) ON DELETE CASCADE
			);
		`,
	},
	{
		ID:   66,
		Name: "create_treasury_transfers_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS treasury_transfers (
				id VARCHAR(255) PRIMARY KEY,
				from_pool VARCHAR(100) NOT NULL,
				to_pool VARCHAR(100) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				amount DECIMAL(36, 18) NOT NULL,
				status VARCHAR(50) NOT NULL,
				required_approvals INT NOT NULL,
				current_approvals INT NOT NULL,
				risk_score DECIMAL(5, 4) DEFAULT 0.0 NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_treasury_trans_status ON treasury_transfers (status);
		`,
	},
	{
		ID:   67,
		Name: "create_treasury_transfer_approvals_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS treasury_transfer_approvals (
				id VARCHAR(255) PRIMARY KEY,
				transfer_id VARCHAR(255) NOT NULL,
				admin_id VARCHAR(255) NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
				FOREIGN KEY (transfer_id) REFERENCES treasury_transfers (id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_treasury_app_trans ON treasury_transfer_approvals (transfer_id);
		`,
	},
	{
		ID:   68,
		Name: "create_liquidity_records_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS liquidity_records (
				id VARCHAR(255) PRIMARY KEY,
				asset VARCHAR(50) NOT NULL,
				depth_bid DECIMAL(36, 18) NOT NULL,
				depth_ask DECIMAL(36, 18) NOT NULL,
				spread DECIMAL(36, 18) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_liquidity_rec_asset ON liquidity_records (asset);
		`,
	},
	{
		ID:   69,
		Name: "create_reserve_accounts_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS reserve_accounts (
				id VARCHAR(255) PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				asset VARCHAR(50) NOT NULL,
				backing_ratio DECIMAL(5, 4) NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   70,
		Name: "create_insurance_fund_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS insurance_fund (
				id VARCHAR(255) PRIMARY KEY,
				asset VARCHAR(50) NOT NULL,
				balance DECIMAL(36, 18) NOT NULL,
				cap_limit DECIMAL(36, 18) NOT NULL,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   71,
		Name: "create_reconciliation_results_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS reconciliation_results (
				id VARCHAR(255) PRIMARY KEY,
				blockchain_verified BOOLEAN NOT NULL,
				database_verified BOOLEAN NOT NULL,
				ledger_verified BOOLEAN NOT NULL,
				wallet_verified BOOLEAN NOT NULL,
				transfers_verified BOOLEAN NOT NULL,
				is_consistent BOOLEAN NOT NULL,
				details TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_reconciliation_res_consistent ON reconciliation_results (is_consistent);
		`,
	},
	{
		ID:   72,
		Name: "create_incidents_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS incidents (
				id VARCHAR(255) PRIMARY KEY,
				title VARCHAR(255) NOT NULL,
				severity VARCHAR(50) NOT NULL,
				status VARCHAR(50) NOT NULL,
				details TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents (status);
		`,
	},
	{
		ID:   73,
		Name: "create_alerts_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS alerts (
				id VARCHAR(255) PRIMARY KEY,
				source VARCHAR(255) NOT NULL,
				message TEXT NOT NULL,
				severity VARCHAR(50) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts (severity);
		`,
	},
	{
		ID:   74,
		Name: "create_fraud_events_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS fraud_events (
				id VARCHAR(255) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				action_type VARCHAR(100) NOT NULL,
				risk_score DECIMAL(5, 4) NOT NULL,
				details TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_fraud_user ON fraud_events (user_id);
		`,
	},
	{
		ID:   75,
		Name: "create_recovery_events_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS recovery_events (
				id VARCHAR(255) PRIMARY KEY,
				component VARCHAR(255) NOT NULL,
				details TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   76,
		Name: "create_monitoring_metrics_records_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS monitoring_metrics_records (
				id VARCHAR(255) PRIMARY KEY,
				metric_name VARCHAR(255) NOT NULL,
				metric_value DECIMAL(36, 18) NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);
		`,
	},
	{
		ID:   77,
		Name: "create_health_status_records_table",
		SQL: `
			CREATE TABLE IF NOT EXISTS health_status_records (
				component VARCHAR(255) PRIMARY KEY,
				status VARCHAR(100) NOT NULL,
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
