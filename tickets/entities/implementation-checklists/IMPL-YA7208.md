---
id: IMPL-YA7208
type: implementation-checklist
title: 'Implementation: Multi-writer support for pgstore (cross-process change feed)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Staged delivery (branch feat/postgres-multiwriter)

- [x] **Stage A — pgstore change feed** (commit 054e1044). Producer emits a
schema-scoped `pg_notify('<origin>:<kind>:<op>:<id>')` inside each write's tx
(the 5 single-statement writes wrapped in a tx per RR-ITQN87); a listener
goroutine (own connection) turns remote notifications into store.Events on the
in-process Subscribe() fan-out, with a seq-watermark catch-up (overlap window)
on connect/reconnect/safety-ticker for durability. Per-store originID filters
self-echoes (RR-97VOON); schema-scoped channel avoids cross-talk (RR-CPZGAK);
listener owned by the store (Open/Close), degrades with a warning if it can't
connect. Tests (DB-gated): cross-process propagation, catch-up recovery,
schema-channel isolation, interleaved delivery, self-notification filter.
- [x] **Stage B — data-entry SSE bridge** (commit 75b14af6). App.startStoreEventBridge
subscribes to store.Subscribe() and broadcasts ENTITY create/update/delete to
browsers (RR-GNS360 entity-only). Removed the 3 inline entity broadcasts in
api_v1 so the bridge is the single source — no double-broadcast (RR-MZOKST).
Also fixes a pre-existing gap (fsstore external edits had no SSE consumer).
Tests: bridge mapping + relation-ignore + single-broadcast (memstore);
cross-process A->B SSE (postgres, DB-gated).
- [x] **Stage C — docs** (commit 1eeac3b1). postgres-backend guide gains a
"Multiple writers" section; CLAUDE.md updated from single-writer to the
multi-writer feed design.

## Development

- [x] Unit tests — bridge mapping/de-dup (memstore); self-notification filter
(handleNotification unit). Integration — cross-process feed + SSE (postgres).
- [x] Integration tests — two pgstore instances on one schema; two dataentry
apps on one DB.
- [x] Happy path — cross-process write -> remote Subscribe() -> SSE broadcast.
- [x] Edge cases — catch-up recovery of missed notifications; self-echo filter;
schema isolation; interleaved/concurrent writes; listener degrade-on-fail.
- [x] Error handling — unparseable NOTIFY -> skip (catch-up is the trust
boundary); listener reconnect with backoff; NOTIFY failure never fails a write.

**Verification evidence:**
- pgstore suite (conformance + fuzz + 5 listener tests) green with `-race`
against live PostgreSQL, deterministic over repeated runs.
- dataentry bridge tests green (default + postgres, `-race`); cross-process
A->B SSE proves AC4 end-to-end.
- All 3 tag combos build; `just ci` green (lint, arch-lint, tests, coverage,
build, docs-check); full default suite 60 packages, no failures.
- Two non-trivial bugs found & fixed during impl: (1) catch-up ran per
notification, re-emitting just-notified writes -> restructured loop so catch-up
runs only on connect/reconnect/ticker; (2) startup catch-up replayed all history
-> prime the watermark at start without emitting.

## Test Quality

- [x] Backend-agnostic bridge tests use memstore (no DB) for the mapping/de-dup
logic; DB-gated tests for the genuinely cross-process behavior.
- [x] Self-echo guarantee tested at the unit level (handleNotification) where
the filter lives — deterministic, not timing-dependent.
- [x] Assertions reference seeded objects, not hardcoded strings.

## Manual Verification

- [x] Ran the full pgstore + dataentry suites against live Postgres.app 15;
cross-process propagation and SSE bridge confirmed.
- [x] Each acceptance criterion has a passing test (AC1-7 + AC4 SSE).

## Quality

- [x] Follows project patterns — mirrors the in-process watcher; consumer-side
interfaces; listener owned by store with clean Close ordering (stop listener
before subscriber channels; uses own conn, not the pool).
- [x] DRY — single notify() producer helper; single catchUp; shared payload
codec in feed.go.
- [x] No security issues — NOTIFY payload is a hint (catch-up is the trust
boundary); unparseable payload skipped, never trusted; channel derived from
current_schema() (validated identifier, quoted for LISTEN); pg_notify args
parameterized.
- [x] No silent failures — listener errors logged; degrade-on-connect-fail warns
loudly; NOTIFY failures intentionally ignored (best-effort hint, documented).
- [x] No debug code left behind.
