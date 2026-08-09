# P0014 RPO & RTO MEASUREMENTS REPORT

- **PostgreSQL Database Instance:**
  - Measured Backup Duration: 2.10s
  - Measured Restoration Duration: 1.14s
  - Target RPO: &lt; 1s | Target RTO: &lt; 10s
- **Redis Cache Instance:**
  - Measured Reconstruction Duration: 0.15s
  - Target RPO: Ephemeral | Target RTO: &lt; 60s
- **Matching Engine Instance:**
  - Measured Journal Replay Duration: 0.05s
  - Target RPO: &lt; 500ms | Target RTO: &lt; 1s
- **Wallet Service Instance:**
  - Measured Lock Acquisition Duration: 0.01s
  - Target RPO: &lt; 100ms | Target RTO: &lt; 10s
