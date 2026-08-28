---
id: TKT-RNBLAC
type: ticket
title: Consolidate cardinality analyzers; stop swallowing CountRelations errors
kind: refactor
priority: high
effort: s
status: done
---

Design doc §12.5 + §12.6 — the suggested first move; unblocks Step 5 and fixes a
real user-facing bug.

`checkMinOutgoing`/`checkMaxOutgoing`/`checkMinIncoming`/`checkMaxIncoming`
(`analysis.go:344-451`) are four near-identical ~25-line functions, each
independently calling `collectEntities` (one relation with one source type scans
the same entities four times). Collapse to one `checkCardinality(ctx, rule,
scope)` parameterised over direction/type-source/bound/comparison; fold
`countOutgoingByType`/`countIncomingByType` into one `countRelations`.

Fix `n, _ := s.deps.Store.CountRelations(...)` (`analysis.go:454,464`): a
backend failure reads as count 0, which for `min_outgoing` manufactures a
violation out of an outage.

**Verified 2026-08-19:** the cited code moved with the analysis-package lift —
it lives in `internal/analysis/analysis.go:345-451` (four checks) and `:453-469`
(the two swallowing count helpers); same shape as described. **Error-policy
decision (per plan PLAN-KJ76OT):** a store error fails the cardinality run
loudly — `CheckCardinality` returns an error and aborts on the first count
failure with entity/relation context; it never reports a violation computed from
a failed count, and `analyze cardinality` / `validate --check cardinality` /
`analyze all` surface it as a command failure.

Prerequisite, not cosmetics: world-awareness changes the subject population,
counting scope, and violation identity — four copies is four chances to diverge
into false violations.
