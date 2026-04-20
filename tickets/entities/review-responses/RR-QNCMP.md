---
id: RR-QNCMP
type: review-response
title: Orphaned per-repo user-state directories accumulate over time with no GC
finding: Over repeated keys init / keys decrypt cycles, stale per-repo dirs accumulate under <base>/rela/repos/. No cleanup command. Cruft over years.
severity: minor
reason: Out of scope for this PR. Document as known limitation. Follow-up ticket can add rela config prune or similar. Not a correctness issue — just disk hygiene.
status: deferred
---
