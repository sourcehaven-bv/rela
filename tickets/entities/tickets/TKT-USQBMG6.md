---
id: TKT-USQBMG6
type: ticket
title: 'Audit service seam and SQL audit log: in-transaction, hash-chained, verifiable'
kind: enhancement
priority: high
effort: xl
tags: security
started: "2026-11-16"
completed: "2026-11-17"
status: done
---

## Description

PR 1 of FEAT-0BH46JQ.

- `audit.Audit` stays the write seam. The read side comes in TKT-X1ZFO04 as a separate capability.
- `sqlaudit`: one SQL implementation over `sqlexec`, adapted for PostgreSQL (`pgaudit`, tenant schema, pg migration 0050) and SQLite (`sqliteaudit`, schema v41).
- Each row carries the hash of the previous row. `rela audit verify` walks the chain and prints the head.
- `rela audit prune --keep-days N` keeps the chain verifiable from an anchor; below 1095 days needs `--allow-short-retention`.
- `rela audit import` appends existing JSONL files.
- appbuild wires the database sink on the sqlite and postgres builds; fs keeps JSONL.
- In this PR records are appended after commit by a background writer (retry, then `audit.write_failed`). Writing inside the store transaction and failing closed is TKT-TU0VZ4Y (DEC-U78WD7Y).
