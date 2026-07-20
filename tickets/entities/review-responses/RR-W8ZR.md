---
id: RR-W8ZR
type: review-response
title: 'Cranky #11: relation-delete error swallowing'
finding: Manager.DeleteEntity and cascadeHost.DeleteEntity silently swallow non-NotFound relation-delete errors — graph can corrupt with no caller-visible error.
severity: significant
reason: Pre-existing bug shipped in TKT-QTNX, not introduced by the reviewed
  change (TKT-IU2S), so fixing it in that PR would have widened scope into the
  entitymanager delete path with its own test surface. Tracked as BUG-C20T
  (still in backlog as of 2026-07-20), which carries the full failure scenario;
  the fix belongs there where the swallow sites can get dedicated regression
  tests.
status: deferred
---
