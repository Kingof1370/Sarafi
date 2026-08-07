# VELYXORA EXCHANGE - DISASTER RECOVERY & JOURNALS
Details platform fault recovery mechanisms.

## Recovery Pipelines
- **Disk-Journal Logging:** Every successful order creation and cancellation is recorded sequentially on disk.
- **Replay Sequence:** On bootstrap or recovery trigger, the journal is parsed and sequentially replayed to rebuild matching book levels cleanly without state desynchronizations.
- **Interruption Re-settlement:** Dual ledger constraints prevent double-settlement or duplicates during re-trigger attempts.
