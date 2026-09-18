---
id: AM-analyze-reports-bare-row-on-faced-type
type: automated-measure
title: analyze reports a bare row on a faced type as stranded
description: Asserts CheckStates reports a stranded-data finding for a bare row on a type declaring faces, and reports none for a bare row on a faceless type. The second half guards against a fix that drops faceDeclared's IsDefault early return outright.
kind: test
location: internal/analysis/states_test.go
status: proposed
---

## Measure

A test asserting that `analysis.CheckStates` reports a finding for an entity
stored at the bare coordinate whose type declares `faces:`, and reports none for
a bare row on a type declaring no faces.

Both halves are needed: the second is what `faceDeclared`'s `IsDefault()` early
return exists to protect, and a fix that drops the early return outright would
start reporting every faceless entity in the project as stranded.

## Why automated

The regression is silent by nature — the check returns "no findings", which is
indistinguishable from a healthy project. `prototypes/perf/project` is a
standing in-tree reproducer (`perfseed` writes every policy at the bare
coordinate while `policy` declares `draft`/`published`), so the fixture to
assert against already exists.
