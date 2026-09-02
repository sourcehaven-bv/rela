---
id: TKT-VAD46Q
type: ticket
title: RelationPredicate.Depth silently clamps to 5 — make the cap explicit in the API
kind: refactor
priority: low
effort: s
status: backlog
---

## Summary

Surfaced by the TKT-5LUGYP code review (its finding #12): a caller passing
`RelationPredicate.Depth: 64` gets a walk silently clamped to 5
(`graphquerynaive.DepthCap`; pgstore's `cappedDepth` mirrors it so SQL and Go
agree). The clamp is documented only in godoc prose — the API's failure mode is
"quietly returns a subset of the correct answer", which is the worst available
option for a predicate also used by ACL read filtering. The gantt subtree fast
path shipped with exactly this bug (fixed by iterating the closure; RR-YRD34H).

## Proposal

Either export the cap as the only legal way to say "as deep as possible" (making
an over-cap literal obviously wrong at the call site), or have the store reject
an out-of-range Depth outright. Audit existing callers
(`internal/acl/readquery.go` already pins to the constant; the gantt now
iterates).
