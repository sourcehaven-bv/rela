---
id: RR-DH5FHU
type: review-response
title: 'Drive-by repair: detached godoc had been attributing the elevation-capability contract to readUsage'
finding: On the base branch the 30-line newElevatedHandle doc block ran straight into `type readUsage struct` with NO blank line between them. Go therefore attached the entire elevation-capability contract — the nil-er/nil-em asymmetry, the three-state fail-closed explanation — to a two-field string-slice struct, while newElevatedHandle itself had no doc comment at all. commentlint does not catch detached-comment drift, so this would have misled readers indefinitely.
severity: minor
resolution: 'Fixed incidentally by this PR''s file split: the comment now sits on newElevatedHandle in elevation.go and readUsage has its own doc. Confirmed by the reviewer against the base branch. Recorded here because the commit message does not mention it — a reader diffing the two revisions would otherwise see the doc block ''move'' with no explanation.'
status: addressed
---

Observation from the TKT-YVREQN code review (cranky-code-reviewer, PR #1467).
Not a defect introduced by the PR — a pre-existing godoc-attribution bug the PR
happens to repair.

Reviewer's verification of the security-critical surface: the elevation gate in
registerBindings is unchanged (`ElevatedManager != nil || ElevatedReader !=
nil`, still nested in `allowWrites`), so structural absence of unwired
capabilities is intact; the nil-em/nil-er asymmetry in newElevatedHandle is
byte-identical (write methods absent when unwired, read methods
present-and-raising);
recordElevatedReads/readUsage/mark/registerElevatedWrites/registerElevatedReads/elevatedGetEntity/elevatedListEntities/elevatedGetRelations
are all byte-identical; 24 elevation tests pass unmodified.

Notably `ctxFn: r.callerCtx` is a bound method value on a pointer receiver, so
it re-reads parentCtx at call time. Had it been written `ctx: r.callerCtx()` the
elevated writes would have lost the caller's Principal and triggered_by — a
silent attribution hole. Worth remembering for any future binding that carries a
context.
