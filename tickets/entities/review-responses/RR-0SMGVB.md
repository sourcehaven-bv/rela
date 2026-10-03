---
id: RR-0SMGVB
type: review-response
title: Failed relations merge never recovers
finding: When mergeRelations returns null relationsDirty is cleared and relationsBase never moves; every later relations save 412s against the same stale token until reload.
severity: minor
reason: The base is left unmoved, so the next relations save retries the merge and recovers once the include resolves. It stays stuck only on a persistent server fault or an unkeyed faced id, which is rare enough to handle when seen.
status: deferred
---
