---
id: RR-VWBUDG
type: review-response
title: New per-level header scan is a new read path and needs a storetest.Counting budget test
finding: |-
    The frontier source gate adds a store query (ListEntityHeaders) plus a gate probe per distinct type, per BFS level. CLAUDE.md requires that "new read paths pin their cost with a storetest.Counting budget test asserting the count is the same at 10 and 50 rows", and the diff had no such test.

    The concern raised was that the 10-pass fixpoint x traverse rules x BFS depth could multiply into many full scans per request, and that on fsstore ListEntities walks the whole entity index per call.
severity: significant
status: addressed
resolution: |-
    Added TestQueryBudget_RecursiveViewTraversalIsSizeIndependent to the existing querybudget_test.go harness, plus a `recursive` view config exercising a Recursive traverse rule (the pre-existing view budget test uses a NON-recursive rule, so it never touched the frontier gate — which is why it kept passing unchanged).

    Measured: 11 store reads at both 10 and 50 rows (GetEntityState=1 ListEntities=2 ListEntityHeaders=1 ListRelations=7), pinned as recursiveViewBudget. The gate contributes one ListEntityHeaders per level walked, and the fixture's n-1 blockers sit at one level, so the count is a function of DEPTH and not of row count — exactly the property the rule exists to enforce. The ungated baseline is the same 11 minus that one header scan.

    The deeper point stands and is accepted rather than dismissed: the cost is bounded by depth (max 10) times distinct types per level, not by result size. Reducing it further would mean having the gate consume headers the loader already fetches, which is a restructuring of the traversal beyond this security fix.
---

## Context

Found by code review of the BUG-9Z20WH fix.
