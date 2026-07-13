---
id: RR-4F3ETV
type: review-response
title: Stale target_not_found warning text now contradicts 422 behavior
finding: In relations_modern.go collectEdgeWarnings, the Phase-A warning for a missing peer still reads 'the edge will be created but reference a missing peer.' Post-fix the write phase hard-422s that case, so the warning is misleading in a mixed batch.
severity: minor
reason: Minor, user-facing text only (no behavior/security impact). Deferred to a quick follow-up polish; reword to 'peer does not exist; this edge will be rejected.' Deferring keeps the security fix minimal.
status: deferred
---
