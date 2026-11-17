---
id: TKT-4I0XFKO
type: ticket
title: sqlite text form of a large real differs from Go in ordered text comparisons
kind: chore
priority: low
effort: s
status: backlog
---

## Description

Under `store.OrderText` an ordered comparison reads a property by its text form.
graphquerynaive uses `fmt.Sprint`, which writes the float 1e19 as `1e+19`.
sqlitestore reads the JSON real and writes it as `1.0e+19`. So `prop <= "10"`
matches the row on sqlite and not on the reference. pgstore writes the jsonb
numeric in full and happens to agree here, but has no pinned contract for reals
either.

Found while extending `storetest.graphDiffValues` for TKT-8HY05WB; the value was
left out of the shared mix.

## Done when

- The text form of a real under OrderText is one documented rule on every backend.
- `graphDiffValues` includes a real beyond int64 and `RunGraphDifferential` passes on sqlite and postgres.
