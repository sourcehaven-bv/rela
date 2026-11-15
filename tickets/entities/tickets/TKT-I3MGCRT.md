---
id: TKT-I3MGCRT
type: ticket
title: Lower ordered and negated desktop notification conditions
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

Follow-up of TKT-EBBHM55. A desktop.yaml condition that `queryplan.LowerScope`
lowers exactly is answered by the store: a rule reads at most
`desktopnotify.MaxMatchesPerRule`+1 headers and the badge is one
`store.CountMatched`. Any other condition still reads every header of its type
that passes the equalities `queryplan.ConditionPrefilters` finds, and judges the
rest in Go (`internal/desktopnotify/evaluate.go#evaluation.scan`, a
`request-path` entry in `internal/archguard/unpagedread_allowlist_test.go`).

The documented example conditions do not lower: `entity.status != 'done' and
entity.due < today()`. `~=` and ordered comparisons against `today()` are
outside what `LowerScope` reads.

Steps:

- Lower `~=` to `store.PropNotEqualOrEmpty` and an ordered comparison of a
date property against `today()` to `PropGreaterEqual`/`PropLessEqual`, with
`today()` resolved at evaluation time from the configured clock. A strict
`<`/`>` has no store operator yet; decide whether to add one.
- Keep the lowering exact (nothing re-checks it). Do not widen `LowerScope`
itself without checking its other callers: `visible:` policies require their
`when:` to lower through it, and CLAUDE.md lists ordered comparisons as refused
there. A desktop-only lowering, or a separate entry point, may fit better.
TKT-I39Y7N4 (push list conditions into the store) covers the same widening for
data-entry lists; share the work.
- Remove the `evaluation.scan` allowlist entry when no condition needs it, or
narrow it.
- Pin with `storetest.AssertScaleFree`, padding with rows that pass the
pushed equalities.
