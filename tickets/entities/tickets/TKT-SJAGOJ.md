---
id: TKT-SJAGOJ
type: ticket
title: 'Relation picker: list entities readable on any face in one request'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

The relation picker lists targets per world. For a faced target type it now
fetches every readable world and merges the rows by id (#1721). Two gaps remain:

- A face that no readable world serves cannot be offered.
- The picker makes one list request per world.

Relation heads are entity-level (DEC-NPZICR), so the picker needs a list mode
that returns every entity the principal can read on some face. It should be
paged, gated through `visibility.Resolver` (`Family`/`ReadableTypes`), and label
each row by the world's served face or else a readable face.

## Acceptance

- One paged request lists every entity readable on some face, including faces no world serves.
- A `storetest.Counting` budget test shows the same read count at 10 and 50 rows.
- The SPA picker uses it and drops the per-world fan-out.
