---
id: RR-LG8QUB
type: review-response
title: 'PR-A: rebuildPropCache loaded the default file per state row - permanent count ghosts'
finding: fsstore's rebuildPropCache iterated the state-keyed index but called the old bare-id loader, so every non-default state re-added the DEFAULT entity's properties to the prop cache on reopen; removeEntityFromCache is default-only, so deletes decremented once and the surplus survived forever — deleted entities kept appearing in suggestion values, drifting further each reopen. Reviewer proved it with a reopen test (open:1 -> open:2 after reopen; ghost value after full delete). The live paths had the IsDefault guard; the rebuild path was the one nothing in the suite exercised.
severity: critical
resolution: rebuildPropCache now skips non-default metas and loads via loadEntityMeta (internal/store/fsstore/index.go). Pinned by the new TestStatePersistence_FamilySurvivesReopen (reopen + count-once + no-ghost-after-delete) and the count-ORDER-sensitive DefaultWorldAggregates conformance case.
status: addressed
---
