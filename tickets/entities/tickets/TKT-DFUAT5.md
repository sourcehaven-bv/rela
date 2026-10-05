---
id: TKT-DFUAT5
type: ticket
title: Add --access-log for per-request timing at info level
kind: enhancement
priority: medium
effort: s
status: ready
description: Log the request record (method/path/status/wall_ms/db_ms) at Info without Debug or SQL logging; never sets Server-Timing.
---

## Description

Operators need per-request timing without Debug logging. `--verbose` is the only
way to get the `request` log record today, and it also logs every SQL statement
with its bound arguments, which puts user data in the central log.

`--access-log` emits the existing `request` record (method, path without query
string, status, `wall_ms`, `queries`, `db_ms`) at Info for every request. It
never sets the `Server-Timing` header: the per-response query count stays a
Debug-only diagnostic because it is an existence side channel to the client.

`--access-log` with `--quiet` refuses to start, since Warn level would drop
every record.

Consumer: the devops perf-report (slow request mail for atlas).

## Acceptance

- [x] With `--access-log` at Info: one `request` record per request, no `Server-Timing` header, no query string.
- [x] Debug plus `--access-log`: header as under Debug, one record at Info.
- [x] Wired through `App.SetAccessLog` and `NewRouter`.
