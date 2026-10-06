---
id: TKT-DFUAT5
type: ticket
title: Add --access-log for per-request timing to syslog or stderr
kind: enhancement
priority: medium
effort: s
started: "2026-10-05"
completed: "2026-10-05"
status: done
description: 'Per-request timing record (method, path, status, wall_ms, queries, db_ms) to a separate access logger: --access-log=syslog (tag rela-access) or stderr. No SQL, no query string; never sets Server-Timing.'
---

## Description

Operators need per-request timing without Debug logging. Today the `request`
record exists only under `--verbose`, which also logs every SQL statement with
its bound arguments, and that puts user data in the central log.

`--access-log=syslog|stderr` sends the existing `request` record (method, path
without query string, status, `wall_ms`, `queries`, `db_ms`) to a separate
logger at Info, for every request.

- `syslog`: the local socket `/dev/log`, tag `rela-access`. Under systemd
this gives its own journal identifier, so the application log (`journalctl -t
rela-server-postgres`) stays free of per-request lines and the access log is
read with `journalctl -t rela-access`.
- `stderr`: for local use.

The access logger is independent of `--verbose`/`--quiet`. It never sets the
`Server-Timing` header: the per-response query count stays a Debug-only
diagnostic because, sent to a client, it is an existence side channel.

The logger reaches the middleware as a `dataentry.WithAccessLog` option to
`App.NewRouter`, not as an `App` setter, so `App` stays under its plimsoll
method cap.

Consumer: the devops perf-report (slow-request mail for atlas).

## Scope

- In: the flag, the separate logger, syslog behind a build tag (rela also
builds for Windows), docs in GUIDE-server-security.
- Out: log files with rotation (journald already rotates and the journal is
already shipped), configurable tag, JSON output.

## Acceptance

- [x] `--access-log=syslog`: one `request` line per request with tag `rela-access`, nothing in the application log.
- [x] `--access-log=stderr`: same line on stderr.
- [x] Lines are written under `--quiet` too.
- [x] No `Server-Timing` header from the access log; Debug behaviour unchanged.
- [x] No query string in the line.
- [x] Unknown value exits 2 at flag parsing; an unreachable syslog socket stops startup.
