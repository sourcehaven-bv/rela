---
id: TKT-1X1X24O
type: ticket
title: Remove the desktop notification Go scan after TKT-4Z2BIRG
kind: chore
priority: low
effort: s
status: backlog
---

## Description

TKT-I3MGCRT made the store answer every desktop.yaml condition that
`queryplan.LowerCondition` lowers. A condition that compares a date with
`today()` still runs on the Go scan in
`internal/desktopnotify/evaluate.go#evaluation.scan`, narrowed by the conjuncts
that lower (`queryplan.ConditionSuperset`). It is a `request-path` entry in
`internal/archguard/unpagedread_allowlist_test.go`.

When TKT-4Z2BIRG lowers date comparisons against `today()`:

- Lower desktop.yaml conditions with today() resolved from the configured
clock, and check that the documented examples lower exactly.
- Decide whether a condition that still does not lower becomes a load error.
- Remove `evaluation.scan`, its allowlist entry and `queryplan.ConditionSuperset`
if nothing else uses it.
- Pin with `storetest.AssertScaleFree`.
