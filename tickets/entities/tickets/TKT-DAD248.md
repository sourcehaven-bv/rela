---
id: TKT-DAD248
type: ticket
title: Merge storetest.Counting and storetest.Breadth into one recorder
kind: refactor
priority: low
effort: s
status: backlog
---

## Description

`internal/store/storetest` now has two store decorators that differ by one `int`
per map entry:

- `Counting` (counting.go) records read calls by method name.
- `Breadth` (breadth.go, TKT-M0WMEE) records how many IDS each batched read was
handed, and already records call counts too.

`Breadth` is a strict superset. Merge them into one `Recorder` exposing
`Calls(method)` and `IDs(method)`.

## Why it is worth doing

A test that wants both properties today must either wrap twice — which
double-counts, since the outer decorator sees the inner's forwarded calls — or
pick one and forgo the other. `internal/dataentry/querybudget_test.go` already
pays this: `TestQueryBudget_NestedSectionIsSizeIndependent` and
`TestQueryBudget_NestedRelationColumnsResolveOverEmittedRowsOnly` build two
separate fixtures and traverse the graph twice to measure two properties of the
same request.

`Counting` has many call sites across the repo, which is why this is a follow-up
rather than part of TKT-M0WMEE. The duplication is cheap to merge now and gets
more expensive as `Breadth` gains call sites.

## Also in scope: fix Counting.Tx's latent data race

`Counting.Tx` (counting.go:181-186) wraps the transaction view in a fresh
`&Counting{Store: view, calls: c.calls}`. The child carries a **zero-value
mutex** but shares the parent's map, so it locks its own lock while writing the
parent's map — synchronizing nothing.

This is latent only because nothing concurrently reads through a `Counting`
parent while a `Tx` is open. It is a real race, not a theoretical one:
reproduced under `-race` with 8 goroutines driving parent and Tx view
concurrently, the equivalent shape in `Breadth` produced 8 `DATA RACE` reports.
`Breadth` was fixed for TKT-M0WMEE (the Tx view delegates to the same recorder
via an unexported `breadthView`, so both halves stay under one mutex);
`Counting` was left alone as out of scope for a test-only ticket. The merge is
the natural moment to fix it, since the merged type needs exactly one correct
implementation.

Race detector is on in CI, so this fires the day someone writes a concurrent
budget test.

## Also worth considering: an assertion helper

Every breadth call site will write the same `if got > limit { t.Errorf(...) }`.
A `recorder.AssertMaxIDs(t, method, limit)` would let the failure message —
which currently names RR-HKHPYG and dumps the full breadth — be written once
rather than copied and degraded. Same for the `Calls` side, where `assertBudget`
in `querybudget_test.go` already plays that role but is local to
`internal/dataentry`.

## Acceptance criteria

1. One decorator records both call counts and id breadth; `Counting` and `Breadth` are gone or aliased.
2. The `Tx` view records against the parent under a single mutex, verified by a concurrency test under `-race`.
3. Every existing `storetest.NewCounting` call site compiles and its pinned budgets are unchanged.
4. The two nested budget tests in `internal/dataentry/querybudget_test.go` share one fixture.
