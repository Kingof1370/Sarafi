# Velyxora Exchange — Multi-Signature Threshold Specification

## 1. Overview
High-security actions (such as system key rotations, bulk wallet transfers, or code updates) must utilize threshold multi-signature approval queues.

---

## 2. Threshold Signature Policy
* **Proposal Ingress:** Admin proposals are submitted into the `SignatureRequest` queue.
* **Partial Signature Collection:** Authorized admins approve the proposal by submitting their individual cryptographic signature.
* **Finalization Threshold:** Once the required threshold count is achieved (e.g., 2-of-3, or 20-of-30 in concurrent tests), the proposal status changes to `COMPLETED`, letting the executor daemon finalize the transaction on the target blockchain network.
* **Audit Trails:** Every approval signature and status update is logged inside database historical tables (`signature_approvals`, `signature_requests`).
