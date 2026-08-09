# Velyxora - SRE Alerting Report

We have implemented an alerting coordinator inside the main API gateway.

## Live Alerts Triggered
We explicitly ran tests verifying alert triggers:
- When a service goes down, a new incident state of status `OPEN` is registered in table `sre_incidents` along with alert rules mapped inside `sre_alerts`.
- When the service heals and becomes accessible, the status transitions automatically to `RESOLVED`.
- Auto-mitigation and recovery flows are fully operational.
