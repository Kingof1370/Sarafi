# Case Management Workflow

Velyxora's manual investigation workflow allows compliance teams to review, escalate, and resolve suspicious activities.

## Investigation Lifecycle
Compliance cases transition through different operational states:
1. **OPEN**: Auto-generated or investigator-initiated cases.
2. **UNDER_REVIEW**: Case active, investigator assigned, alerts linked.
3. **ESCALATED**: Transferred to senior compliance authority.
4. **RESOLVED**: Verified with legal evidence (e.g. salary slips). Associated alerts are cleared.
5. **DISMISSED**: Marked as false positive. Alerts dismissed.
6. **BLOCKED**: User confirmed bad actor. Permanent restriction applied.

## Audit Trails and Notes
Investigators can add persistent notes documenting their findings. Every single state transition, resolution note, and authorization change is captured in the compliance audit trail.
- Access to cases requires server-side `risk:write` permission.
- Sensitive resolution updates are protected.
