# P0014 HIGH AVAILABILITY REPORT

- **Redundancy Models:** High availability achieved using stateless active-active clusters for API gateways, Multi-AZ replication for databases and Redis caches, and coordinated active-standby models for stateful workers.
- **Exclusivity Enforcement:** Coordinated single-writer models achieved utilizing Postgres Transactional Advisory Locks (`pg_try_advisory_xact_lock`) to eliminate split-brain write conditions.
- **Partition Assignments:** Kafka brokers handle consumer group balancing for downstream event streams.
