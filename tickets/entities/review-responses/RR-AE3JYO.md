---
id: RR-AE3JYO
type: review-response
title: Field-level write restrictions not enforced on attachment writes
finding: writePreflight checks only row-level update; FieldGate is AllowAll in production, so a non-writable file field can be replaced via MCP (and via the web routes).
severity: minor
reason: Pre-existing on every write surface (web PUT/DELETE and update_entity alike); tracked by TKT-0XL8MF (policy-backed FieldWriteGate). Fixing it only for MCP would split the policy.
status: deferred
---
