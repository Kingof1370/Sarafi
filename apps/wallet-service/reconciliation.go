package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"velyxora/packages/common"
	"velyxora/packages/custody"
	"velyxora/packages/database"
	"velyxora/packages/logger"
)

// ReconciliationIssue logs discrepancies detected during audit runs
type ReconciliationIssue struct {
	Layer    string
	Severity string
	Asset    string
	Details  string
}

// ReconciliationEngine runs five-layer ledger audits statefully
type ReconciliationEngine struct {
	mu           sync.Mutex
	db           *database.DB
	log          *logger.Logger
	custody      *custody.CustodyEngine
	localIssues  []ReconciliationIssue
	lastRunError error
}

func NewReconciliationEngine(db *database.DB, ce *custody.CustodyEngine, log *logger.Logger) *ReconciliationEngine {
	return &ReconciliationEngine{
		db:          db,
		custody:     ce,
		log:         log,
		localIssues: make([]ReconciliationIssue, 0),
	}
}

// RunReconciliationAudit performs the five-layered audit checks concurrently and records issues
func (re *ReconciliationEngine) RunReconciliationAudit(ctx context.Context, asset string) (string, []ReconciliationIssue, error) {
	re.mu.Lock()
	defer re.mu.Unlock()

	re.log.Info("Starting five-layer reconciliation audit", "asset", asset)

	runID := fmt.Sprintf("rec_%d", time.Now().UnixNano())
	issues := make([]ReconciliationIssue, 0)

	if re.db != nil {
		_, err := re.db.Pool.Exec(ctx,
			"INSERT INTO reconciliation_runs (id, status, started_at) VALUES ($1, 'RUNNING', NOW())",
			runID)
		if err != nil {
			return "", nil, err
		}
	}

	// ----------------------------------------------------
	// LAYER 1: Blockchain State Lookup
	// ----------------------------------------------------
	adapter, err := common.GetBlockchainAdapter(asset)
	blockchainTotal := 0.0
	if err == nil && adapter.GetMode() == common.ModeSimulation {
		sim := common.GetSimulator(asset)
		// Sum all balances across simulated addresses
		blockchainTotal = sim.SumAllBalances(asset)
	}

	// ----------------------------------------------------
	// LAYER 2: DB Records Lookup (Deposits & Withdrawals)
	// ----------------------------------------------------
	dbDepositTotal := 0.0
	dbWithdrawalTotal := 0.0
	if re.db != nil {
		_ = re.db.Pool.QueryRow(ctx,
			"SELECT COALESCE(SUM(amount), 0.0) FROM deposits WHERE asset = $1 AND status = 'COMPLETED'",
			asset).Scan(&dbDepositTotal)

		_ = re.db.Pool.QueryRow(ctx,
			"SELECT COALESCE(SUM(amount), 0.0) FROM withdrawals WHERE asset = $1 AND status = 'COMPLETED'",
			asset).Scan(&dbWithdrawalTotal)
	}

	// Net DB Flow
	dbNetFlow := common.RoundToPrecision(dbDepositTotal-dbWithdrawalTotal, 8)

	// ----------------------------------------------------
	// LAYER 3: Wallet Balances Lookup (Balances Table)
	// ----------------------------------------------------
	walletTotal := 0.0
	if re.db != nil {
		_ = re.db.Pool.QueryRow(ctx,
			"SELECT COALESCE(SUM(total), 0.0) FROM balances WHERE asset = $1",
			asset).Scan(&walletTotal)
	}
	walletTotal = common.RoundToPrecision(walletTotal, 8)

	// ----------------------------------------------------
	// LAYER 4: Ledger Entry Totals (Double-Entry Balance Check)
	// ----------------------------------------------------
	ledgerDebitTotal := 0.0
	ledgerCreditTotal := 0.0
	if re.db != nil {
		_ = re.db.Pool.QueryRow(ctx,
			"SELECT COALESCE(SUM(amount), 0.0) FROM ledger_entries WHERE asset = $1 AND type = 'DEBIT'",
			asset).Scan(&ledgerDebitTotal)

		_ = re.db.Pool.QueryRow(ctx,
			"SELECT COALESCE(SUM(amount), 0.0) FROM ledger_entries WHERE asset = $1 AND type = 'CREDIT'",
			asset).Scan(&ledgerCreditTotal)
	}

	ledgerBalanceDiff := common.RoundToPrecision(ledgerDebitTotal-ledgerCreditTotal, 8)

	// ----------------------------------------------------
	// LAYER 5: Institutional Custody State Check
	// ----------------------------------------------------
	custodyTotal := 0.0
	if re.custody != nil {
		custodyTotal += re.custody.GetVaultBalance(custody.RoleHot, asset)
		custodyTotal += re.custody.GetVaultBalance(custody.RoleWarm, asset)
		custodyTotal += re.custody.GetVaultBalance(custody.RoleCold, asset)
		custodyTotal += re.custody.GetVaultBalance(custody.RoleTreasury, asset)
	}

	// ----------------------------------------------------
	// DISCREPANCY DETECTIONS & SEVERITY LOGGING
	// ----------------------------------------------------

	// Audit Rule 1: Wallet vs Ledger integrity
	if re.db != nil && walletTotal != dbNetFlow {
		issue := ReconciliationIssue{
			Layer:    "LAYER_3_VS_LAYER_2",
			Severity: "CRITICAL",
			Asset:    asset,
			Details:  fmt.Sprintf("Discrepancy: Sum of user balances (%f) != Net flow of completed DB transactions (%f)", walletTotal, dbNetFlow),
		}
		issues = append(issues, issue)
	}

	// Audit Rule 2: Double-entry debits must balance credits
	// Note: in a pure double-entry ledger of accounts, sum of debits minus sum of credits is exactly zero.
	if re.db != nil && ledgerBalanceDiff != 0.0 {
		issue := ReconciliationIssue{
			Layer:    "LAYER_4_DOUBLE_ENTRY",
			Severity: "HIGH",
			Asset:    asset,
			Details:  fmt.Sprintf("Discrepancy: Ledger debit/credit trial balance is out of balance by %f", ledgerBalanceDiff),
		}
		issues = append(issues, issue)
	}

	// Audit Rule 3: Blockchain balance vs database net-flow mismatch
	if adapter != nil && adapter.GetMode() == common.ModeSimulation && blockchainTotal < walletTotal {
		issue := ReconciliationIssue{
			Layer:    "LAYER_1_VS_LAYER_3",
			Severity: "WARNING",
			Asset:    asset,
			Details:  fmt.Sprintf("Discrepancy: Blockchain physical balance (%f) is less than registered wallet liabilities (%f)", blockchainTotal, walletTotal),
		}
		issues = append(issues, issue)
	}

	// Audit Rule 4: Custody assets availability
	if custodyTotal < dbWithdrawalTotal {
		issue := ReconciliationIssue{
			Layer:    "LAYER_5_CUSTODY",
			Severity: "WARNING",
			Asset:    asset,
			Details:  fmt.Sprintf("Discrepancy: Segregated custody vault assets (%f) is less than total client withdrawals (%f)", custodyTotal, dbWithdrawalTotal),
		}
		issues = append(issues, issue)
	}

	// ----------------------------------------------------
	// PERSISTENCE & SCHEDULING WRAP
	// ----------------------------------------------------
	status := "SUCCESS"
	if len(issues) > 0 {
		status = "WARNING"
	}

	if re.db != nil {
		detailsBytes, _ := json.Marshal(issues)
		_, _ = re.db.Pool.Exec(ctx,
			"UPDATE reconciliation_runs SET status = $1, completed_at = NOW(), details = $2 WHERE id = $3",
			status, string(detailsBytes), runID)

		for _, issue := range issues {
			issueID := fmt.Sprintf("iss_%d", time.Now().UnixNano())
			_, _ = re.db.Pool.Exec(ctx,
				"INSERT INTO reconciliation_issues (id, run_id, layer, severity, asset, details, timestamp) VALUES ($1, $2, $3, $4, $5, $6, NOW())",
				issueID, runID, issue.Layer, issue.Severity, issue.Asset, issue.Details)
		}
	} else {
		re.localIssues = issues
	}

	re.log.Info("Reconciliation audit finished", "asset", asset, "status", status, "issues_count", len(issues))
	return runID, issues, nil
}

// StartReconciliationScheduler launches periodic reconciliation loops in the background
func (re *ReconciliationEngine) StartReconciliationScheduler(ctx context.Context, wg *sync.WaitGroup, interval time.Duration) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		re.log.Info("Starting Reconciliation background scheduler...")
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				re.log.Info("Reconciliation background scheduler stopped.")
				return
			case <-ticker.C:
				// Audit BTC, ETH, and SOL
				for _, asset := range []string{"BTC", "ETH", "SOL"} {
					_, _, err := re.RunReconciliationAudit(ctx, asset)
					if err != nil {
						re.log.Error("Scheduled reconciliation run failed", "asset", asset, "err", err)
					}
				}
			}
		}
	}()
}
