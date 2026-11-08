---
id: TKT-DAUS290
type: ticket
title: 'rela test: let the operator choose the test backend (sqlite/postgres) with parallel case isolation'
kind: enhancement
priority: low
effort: l
status: backlog
---

## Description

v1 runs every case on memstore. Operators on sqlite or postgres may want tests
on their own backend, to catch backend-specific behaviour (search ranking, `Tx`
semantics, SQL graph queries).

Design questions:
- **Isolation per case:** a database or schema per case, or a transaction rolled back after each case.
- **Parallelism:** memstore cases can run in parallel safely; a shared database needs per-case schemas, or serial execution.
- **Configuration:** a `tests/rela-test.yaml` or flag naming the backend and a test DSN, never the production DSN (env only, per the DSN rule).
- **Cost:** setting up a schema per case on postgres versus test runtime.
