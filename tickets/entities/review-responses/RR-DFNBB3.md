---
id: RR-DFNBB3
type: review-response
title: Cap without a cursor is a dead end, not a bound
finding: 'PLAN-VZXHRJ specified count/total/truncated but no way to fetch the next page. That tells a script it was cut off while giving it no means to continue — so the plan compensated by requiring in-tree whole-set callers (generate-docs.lua, dev-status.lua, related.lua) to check truncated and FAIL. That is papering over a missing feature with an error message. These loops are trivially pageable (generate_entity_type writes one file per entity, no cross-entity state), so the right surface is a cursor: the cap becomes a PAGE SIZE, memory stays bounded per call, and complete iteration remains possible. Raised by the user, not by the review — the review accepted the fail-loudly mitigation without questioning why a bound needed one.'
severity: significant
resolution: 'Deferred to DEC-IYHLNF stage 2, which is blocked on store-side paging (store.GraphQueryer has no Limit/Cursor). The finding is correct and drove a real design change: the plan''s ''fail loudly on truncation'' mitigation was dropped as papering over a missing feature. Stage 1 (TKT-DLYBWE) instead makes `cursor` an ERROR rather than accepted-and-ignored, so no script can be written against a fake cursor that silently never advances — which is what makes adding real paging in stage 2 purely additive. Interim gap accepted and documented: a type over 2000 rows is unreachable until then. Verified no in-tree type approaches that (largest is 933).'
reason: 'Blocked on store-side paging: store.GraphQueryer has no Limit/Cursor, verified against develop 2026-07-29 (nothing merged, nothing open). A binding-level cursor has nothing to sit on, and the two fakeable interims are both worse than waiting — an inert cursor makes the idiomatic paging loop infinite, an offset-backed one skips and duplicates rows under concurrent writes. Scheduled for DEC-IYHLNF stage 2 once the store capability lands. Interim gap is bounded and measured: a type over 2000 rows is unreachable, and the largest in-tree type is 933.'
status: deferred
---

Consequence: this makes the store-side paging gap **blocking rather than
deferrable**. `GraphQueryer` has no `Limit`/`Cursor`, so a binding-level cursor
has nothing to sit on.

Faking it with an offset over a materialized set is NOT an acceptable interim:
offsets over an ordering that can change between calls silently skip or
duplicate rows mid-iteration. For a doc generator that is precisely the
corruption the cap exists to prevent — worse than an honest ceiling, because it
fails silently rather than loudly.

Two viable sequencings:

- **(a) Wait for the store's paged `GraphQuery`**, then do this properly with a
cursor. Whole-set callers get a `repeat ... until not cursor` loop.
- **(b) Ship cap-and-report now**, cursor as a follow-up. Nothing in-tree breaks
today (largest type is 909 vs a 5000 cap), but the binding's surface changes
twice.

Recommendation: **(a)**. The cursor is the permanent design; shipping (b) means
two breaking changes to a documented binding instead of one, and the second
would land weeks later on scripts already written against the first.
