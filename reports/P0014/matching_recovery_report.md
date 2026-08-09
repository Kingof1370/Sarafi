# P0014 MATCHING ENGINE RECOVERY REPORT

- **In-Memory Order Book RPO:** Zero order loss.
- **Engine Recovery RTO:** &lt; 1s (journal replay).
- **Primary Single Writer Lock:** Implemented programmatically using PostgreSQL Transaction-Scoped Session advisory locks to prevent dual active matching books and avoid split-brain execution.
- **Verification Tests:** Handled inside `TestObservabilityDisasterRecovery` and `TestP0014_DisasterRecovery_EndToEnd`.
