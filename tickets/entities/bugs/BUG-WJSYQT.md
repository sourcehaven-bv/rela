---
id: BUG-WJSYQT
type: bug
title: List filter and sort evaluate hidden fields (value disclosure)
description: List filter[] and sort evaluate fields the principal cannot see, so a hidden value can be recovered by search.
priority: high
status: backlog
---

## Description

List `filter[]` and `sort` evaluate properties the principal cannot see.
`applyV1Filters` (internal/dataentry/api_v1.go:2176), the sort (:2344, :2378)
and the pushdown in listpushdown.go never check field visibility.

Reproduced by the TKT-0M4H0S security review on the golden corpus fixture
(memstore), as the viewer, whose `secret` and `note` are hidden by `visible:`:
- `?filter[secret]=bravo-secret` returns TKT-2 (total 1); `=nope` returns 0.
- `?filter[secret][contains]=brav` matches, `bx` does not; `[gt]`/`[lt]` work, so a binary search recovers the full value.
- `?sort=secret` orders by the hidden raw value.

This is the value disclosure RR-5KEF8E rejected for gantt and BUG-IPO4UH closed
for search. pgstore pushes the same query (not yet verified there).

## Fix

Reject `filter[]` and `sort` on a property the principal cannot see
unconditionally (400). For a `when:`-conditional field, reject or evaluate on
redacted rows after the gate, never in the pushdown. Pin with corpus cases and a
unit test per operator.
