# P0014 FAILOVER REPORT

- **API Instance Failover:** Stateless load-balancing routing redirects connection requests dynamically. Downtime: &lt; 2s.
- **Database Server Failover:** Hot-standby RDS replica promoted to primary writing state automatically. Downtime: &lt; 10s.
- **Redis Node Failover:** Promotes replica; backend triggers asynchronous idempotent cache reconstruction. Downtime: &lt; 60s.
- **Matching Engine Instance Failure:** Standby process acquires transaction advisory lock and begins replaying local journals. Downtime: &lt; 1s.
