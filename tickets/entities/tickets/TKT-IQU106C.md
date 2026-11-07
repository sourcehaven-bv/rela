---
id: TKT-IQU106C
type: ticket
title: Store capability for range fingerprints
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Add an optional store capability for set reconciliation (FEAT-CMVZQFI):

- `RangeFingerprint(ctx, scope, lo, hi) -> {count, fp}`
- `ListRange(ctx, scope, lo, hi) -> [(address, hash)]`

The scope is the set of types (and faces) plus the ACL row predicate, applied
inside the query the way search applies it. The consumer defines the interface
at the call site.

- pgstore and sqlitestore compute the sum in SQL.
- A naive reference implementation, a conformance suite in `storetest`, and a randomized differential test against it (the `graphquerynaive` pattern).
- Defined here only for rows where the stored hash is the view hash; redacted types come in the next ticket.

## Acceptance

- All backends pass the conformance and differential tests.
- A budget test pins the cost of one range query.
