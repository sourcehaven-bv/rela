---
id: RR-485BI9F
type: review-response
title: Tree-hash test skips channels, IdP roles, realtime and script automations
finding: The subsystems with MemFS special cases are not exercised by the tree-hash test.
severity: minor
reason: Channels and IdP roles need a realtime or IdP session the fixture does not have; their MemFS branches go through inMemoryFS which TestInMemoryFS_SelectsProcessLocks covers. Script automations depend on the output-dir seam in TKT-NRNDGZR.
status: deferred
---
