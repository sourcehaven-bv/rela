---
id: TKT-MXZ6FBN
type: ticket
title: Fold id prefix case in gap analysis
kind: chore
priority: low
effort: s
status: backlog
description: Gap analysis keys id prefixes by their declared case and looks them up by the case of each id, so a type declared nt- loses its type badge and a manual doc- type's DOC-12 counts as a sequence gap.
---

## Description

`internal/dataentry/analyze.go` (around lines 389, 415 and 443) and
`internal/analysis/analysis.go` (around lines 344 and 363) build their prefix
maps from the declared `id_prefix` and look them up with the prefix parsed from
each id. Since BUG-4OPP525 the prefix check folds case, so ids whose prefix case
differs from the schema are valid, and these maps miss them:

- gap rows for a type declared `nt-` get an empty entity type;
- the manual-type skip set misses `DOC-12` under `doc-`, so it is reported
as a sequence gap.

## Expected

Key the maps by the upper-cased prefix and look them up the same way. Group
`prefixGroups` by the upper-cased prefix so `nt-1` and `NT-2` form one sequence.
Add a table row per case.
