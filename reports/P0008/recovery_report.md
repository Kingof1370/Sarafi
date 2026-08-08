# Velyxora Phase P0008 Recovery Report

## Recovery Testing Summary

We tested system resiliency after various simulated hardware and database restarts.

### Resiliency Checklists
1. **Database / PG Restart**: Background scanner tracks block heights statefully in `blockchain_scanner_states` and correctly resumes from the last scanned height on database reconnection.
2. **Worker Restart (Pending TX Resumption)**: On service startup, `recoverPendingWithdrawals` correctly scans the database for `BROADCASTING` or `CONFIRMING` withdrawals and spawns monitoring threads to ensure finality is handled.
3. **Kafka Restart**: Both scanner and worker implement robust error logs and retry queue publishers when Kafka brokers are momentarily offline.
