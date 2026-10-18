---
id: RR-X0POFN
type: review-response
title: Stale-pending release is gated on message_ops.remove and has no fallback
finding: C3 ReleaseAction uses the message_ops.remove gate. A valid config that declares actions but no remove leaves a crashed promote pending forever, and D1's whole mitigation rests on release. Fall back to the action's own gate, a lifecycle gate, or an operator CLI path, and validate or warn at load when actions exist without any release path.
severity: significant
resolution: Release uses message_ops.remove when declared and otherwise the action's own gate. Pending can only stick on fs (D1).
status: addressed
---
