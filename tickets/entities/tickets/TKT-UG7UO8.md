---
id: TKT-UG7UO8
type: ticket
title: 'storeutil.TopValues: hoist the triplicated property-value ranking (one copy had already drifted)'
kind: refactor
priority: low
effort: xs
status: in-progress
---

## Description

The sort-and-truncate tail of `PropertyValues` is byte-identical across all
three backends — except for one character that had already drifted:

| File | Pre-allocation |
|---|---|
| `fsstore/entity.go:193` | `make([]string, 0, limit)` |
| `memstore/memstore.go:386` | `make([]string, 0, limit)` |
| `pgstore/entity.go:226` | `make([]string, 0, len(sorted))` |

`limit == 0` means **all values** (`for i := 0; i < len(sorted) && (limit == 0
|| i < limit); i++`), so fs/mem pre-allocate **zero** for the unlimited case
and grow by repeated reallocation. pgstore is correct.

Minor on its own. It matters because a fourth backend inherits whichever copy
its author happens to read, and the drift proves that is a real risk rather than
a hypothetical one — this is the third copy of twenty lines and it has already
gone wrong once.

## Scope

IN: `storeutil.TopValues(counts map[string]int, limit int) []string`; all three
backends delegate; unit tests for the ranking, the `limit <= 0` convention, and
the allocation behaviour that drifted.

OUT: any change to `PropertyValues` semantics. The counting half stays
per-backend — fsstore reads a prop cache, memstore scans entities, pgstore runs
SQL — only the shared ranking tail is hoisted.

## Acceptance criteria

1. One implementation in `storeutil`, three delegating call sites, `sort` no
longer imported by any of the three files.
2. `limit <= 0` returns all values; the result is pre-allocated to the result
size, not to `limit`.
3. Ties broken alphabetically so the result is deterministic across backends
and runs (Go randomizes map iteration).
4. Empty and nil `counts` yield a non-nil empty slice — callers marshal it
straight to JSON, where nil encodes as `null` rather than `[]`.
5. All three conformance suites pass unchanged, including pgstore against a
live database.
