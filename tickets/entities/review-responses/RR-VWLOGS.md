---
id: RR-VWLOGS
type: review-response
title: Fail-closed branches in the new gates dropped entities silently, unlike every sibling gate
finding: |-
    Three drop-everything branches produced no operator signal: the frontier gate's per-type probe failure, ids whose header never arrived, and the collection-load gate's probe failure. Sibling code (visiblereader.go) logs these, with the stated rationale that an operator should see the cause rather than "a silently-empty include block".

    Also flagged: testing the loop-invariant `gerr != nil` INSIDE the per-id loop made the fail-closed branch read as conditional, inviting a later "simplification" that removes it.
severity: minor
status: addressed
resolution: |-
    Both gates now slog.Warn on a probe failure and on a header-scan fault, naming the type and id count. The gerr check is hoisted out of the per-id loop into an early branch that deletes the whole type and continues, which is both clearer and where the log naturally belongs.

    Ids that resolve to no row are still dropped without a log: under a world that is the ordinary `otherwise: exclude` verdict and under the default world it is a dangling edge. Both are normal, per-row, and were silent before this change — logging them would be noise on every page. The header-scan FAULT (the abnormal case) is logged.
---

## Context

Found by code review of the BUG-9Z20WH fix.
