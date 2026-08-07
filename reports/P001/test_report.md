# VELYXORA EXCHANGE - P0001 TEST REPORT
Generated on: 2026-08-07 08:00:00
Execution ID: P0001

## 1. Unit & Integration Test Summary
All test files within the monorepo workspace have been executed successfully against active, real local Docker container dependencies (PostgreSQL, Redis, and Kafka).

### Execution Command
`go test -v ./tests/... ./apps/backend/... ./apps/matching-engine/... ./apps/wallet-service/... ./packages/common/... ./packages/database/... ./packages/logger/... ./packages/security/...`

### Output Logs
```text
=== RUN   TestAuthMiddleware
--- PASS: TestAuthMiddleware (0.00s)
=== RUN   TestWalletWithdrawalValidationAPI
--- PASS: TestWalletWithdrawalValidationAPI (0.00s)
=== RUN   TestWSGatewaySubscriptionFlow
--- PASS: TestWSGatewaySubscriptionFlow (0.00s)
PASS
ok  	velyxora/apps/backend	0.024s

=== RUN   TestOrderBookMatching
--- PASS: TestOrderBookMatching (0.00s)
PASS
ok  	velyxora/apps/matching-engine	0.012s

=== RUN   TestRiskEngine
--- PASS: TestRiskEngine (0.00s)
=== RUN   TestFeesEngine
--- PASS: TestFeesEngine (0.00s)
=== RUN   TestOMS
--- PASS: TestOMS (0.00s)
=== RUN   TestMatcher
--- PASS: TestMatcher (0.00s)
=== RUN   TestExecutionEngine
--- PASS: TestExecutionEngine (0.00s)
=== RUN   TestPositionEngine
--- PASS: TestPositionEngine (0.00s)
=== RUN   TestMarketServices
--- PASS: TestMarketServices (0.00s)
=== RUN   TestSettlementEngine
--- PASS: TestSettlementEngine (0.00s)
=== RUN   TestMatchingLatencyPercentiles

--- VELYXORA LATENCY PERCENTILES ---
P50 (Median) Latency: 465ns
P95 Latency:          636ns
P99 Latency:          8.372µs
------------------------------------
--- PASS: TestMatchingLatencyPercentiles (0.00s)
=== RUN   TestAdvancedRiskValidationAndProtections
--- PASS: TestAdvancedRiskValidationAndProtections (0.00s)
=== RUN   TestSelfTradePreventionTriggers
--- PASS: TestSelfTradePreventionTriggers (0.00s)
=== RUN   TestAssetReservationsAndPnL
--- PASS: TestAssetReservationsAndPnL (0.00s)
=== RUN   TestLiquidityAnalytics
--- PASS: TestLiquidityAnalytics (0.00s)
=== RUN   TestMarketSurveillanceWashTrading
--- PASS: TestMarketSurveillanceWashTrading (0.00s)
=== RUN   TestMarketSurveillanceVelocityAbuse
--- PASS: TestMarketSurveillanceVelocityAbuse (0.00s)
=== RUN   TestFeeEngineVIPLevels
--- PASS: TestFeeEngineVIPLevels (0.00s)
=== RUN   TestFeeEngineReferrals
--- PASS: TestFeeEngineReferrals (0.00s)
=== RUN   TestFeeEnginePromotionalOverrides
--- PASS: TestFeeEnginePromotionalOverrides (0.00s)
=== RUN   TestObservabilityHealthprobers
--- PASS: TestObservabilityHealthprobers (0.00s)
=== RUN   TestObservabilityBackups
--- PASS: TestObservabilityBackups (0.00s)
=== RUN   TestObservabilityDisasterRecovery
--- PASS: TestObservabilityDisasterRecovery (0.00s)
=== RUN   TestOMSValidators
--- PASS: TestOMSValidators (0.00s)
=== RUN   TestOMSStateMachineTransitions
--- PASS: TestOMSStateMachineTransitions (0.00s)
=== RUN   TestOMSRouterWorkflows
--- PASS: TestOMSRouterWorkflows (0.00s)
=== RUN   TestOMSStressConcurrency
--- PASS: TestOMSStressConcurrency (0.10s)
PASS
ok  	velyxora/apps/matching-engine/engine	0.117s

=== RUN   TestBalanceEngineDoubleEntryReconciliation
--- PASS: TestBalanceEngineDoubleEntryReconciliation (0.00s)
PASS
ok  	velyxora/apps/wallet-service	0.014s

=== RUN   TestBlockchainAdapters
--- PASS: TestBlockchainAdapters (0.00s)
=== RUN   TestMockCommonSetup
--- PASS: TestMockCommonSetup (0.00s)
=== RUN   TestRateLimiter
--- PASS: TestRateLimiter (0.60s)
PASS
ok  	velyxora/packages/common	0.615s

=== RUN   TestDBConnectionParsing
--- PASS: TestDBConnectionParsing (0.00s)
PASS
ok  	velyxora/packages/database	0.008s

=== RUN   TestLoggerJSON
--- PASS: TestLoggerJSON (0.00s)
PASS
ok  	velyxora/packages/logger	0.005s

=== RUN   TestHashAndPassword
--- PASS: TestHashAndPassword (0.25s)
=== RUN   TestJWTGenerationAndValidation
--- PASS: TestJWTGenerationAndValidation (0.00s)
=== RUN   TestVerifyAPIKeySignature
--- PASS: TestVerifyAPIKeySignature (0.00s)
=== RUN   TestValidatePasswordStrength
--- PASS: TestValidatePasswordStrength (0.00s)
=== RUN   TestSanitizeInput
--- PASS: TestSanitizeInput (0.00s)
PASS
ok  	velyxora/packages/security	0.257s

=== RUN   TestCoreOrderBookEngineIntegration
--- PASS: TestCoreOrderBookEngineIntegration (0.00s)
=== RUN   TestRealInfrastructureE2E
    real_infrastructure_test.go:29: Connecting to PostgreSQL at localhost:5432...
    real_infrastructure_test.go:36: PostgreSQL connected! Running schema migrations...
    real_infrastructure_test.go:42: Migrations successfully applied! Verifying applied migrations count...
    real_infrastructure_test.go:48: Found 71 applied schema migrations in the database.
    real_infrastructure_test.go:54: Executing double-entry balance updates with row locking (SELECT FOR UPDATE)...
    real_infrastructure_test.go:134: Row locking balance transaction committed successfully!
    real_infrastructure_test.go:163: Connecting to Redis at localhost:6379...
    real_infrastructure_test.go:173: Setting cache key...
    real_infrastructure_test.go:181: Getting cache key...
    real_infrastructure_test.go:190: Deleting cache key...
    real_infrastructure_test.go:197: Connecting to Kafka at localhost:9092...
    real_infrastructure_test.go:220: Publishing message to Kafka...
    real_infrastructure_test.go:226: Consuming published message from Kafka...
    real_infrastructure_test.go:237: Received message from Kafka with key: ord_test_kafka_999
    real_infrastructure_test.go:262: Kafka Pub/Sub transaction verified successfully!
--- PASS: TestRealInfrastructureE2E (10.05s)
=== RUN   TestSecurityRegistrationVerification
--- PASS: TestSecurityRegistrationVerification (0.00s)
PASS
ok  	velyxora/tests	10.065s
```

## 2. Test Verification Status
- **PostgreSQL Database Verification:** PASS (Real Postgres connection, migration checks, dual user INSERTs, SELECT FOR UPDATE row locks, balance subtractions, debit/credit entries, and transactional commit succeed perfectly).
- **Redis Cache Verification:** PASS (Real Redis client connection, Set cache, Get cache comparison, and key Del succeed perfectly).
- **Kafka Pub/Sub Verification:** PASS (Real Kafka broker producer and consumer connections, topic publication, background event stream consumption and key matching succeed perfectly).
- **Test Suite Status:** PASS (All unit, integration, security, and infrastructure tests succeeded).
- **Exit Code:** 0
