---
id: RR-QU6BTDN
type: review-response
title: No test for the sqlite fallback from a path step
finding: A step Through with a property name the sqlite builder cannot render should fall back to RollupGeneric; no test pins it.
severity: minor
reason: The unsafe flag is shared with the existing Related fallback which is already pinned; a step-specific case is follow-up hardening.
status: deferred
---
