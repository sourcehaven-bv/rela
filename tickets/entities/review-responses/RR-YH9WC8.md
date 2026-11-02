---
id: RR-YH9WC8
type: review-response
title: Several-sessions and reload cases need one sentence each
finding: A message type belongs to one channel type, and so to at most one session. The several-sessions case therefore cannot occur, but the plan does not say so. Reload is listed as an edge case without a test. It is covered by construction, because Describe and Read each load one snapshot, MessageType.clone copies derived, and the guide's refresh event makes clients reread _schema/channels. State this instead of listing it as tested.
severity: nit
resolution: "addressed\tPlan states that several sessions cannot occur (one channel type per message type) and that reload is covered by construction (one snapshot per call, clone copies derived, refresh event), not by a new test."
status: addressed
---
