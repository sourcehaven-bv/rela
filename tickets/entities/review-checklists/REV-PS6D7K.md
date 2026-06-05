---
id: REV-PS6D7K
type: review-checklist
title: 'Review: Multi-writer support for pgstore (cross-process change feed)'
status: done
---

&lt;!-- @managed: claude-workflow v1 --&gt;

## Automated Checks

- [x] All tests pass (`just ci` exit 0; postgres suite `go test -race -tags postgres` green against live DB; PR #898 Test + Postgres Backend jobs pass)
- [x] Lint clean (PR #898 Lint job pass)
- [x] Coverage maintained (`just coverage-check` in `just ci`: Summary 0 errors)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer + go-architect) — round 1
- [x] All critical review-responses addressed (RR-CPZGAK, RR-ITQN87)
- [x] All significant review-responses addressed (RR-97VOON, RR-GNS360, RR-11KW9M, RR-4GMZD4, RR-9UGZ67, RR-NYGRRG)
- [x] Self-reviewed the diff for unrelated changes — also reviewed via `/crit` (round 2, approved, 0 outstanding comments)

**Review Responses:** RR-CPZGAK, RR-ITQN87 (critical); RR-97VOON, RR-GNS360,
RR-11KW9M, RR-4GMZD4, RR-9UGZ67, RR-NYGRRG (significant); RR-1QTG37, RR-MZOKST
(minor). All `addressed`.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (write committed by process A delivered to process B's Subscribe within bounded delay): **PASS** — `TestCrossProcessPropagation`, `TestInterleavedWritesAllDelivered`.
- AC2 (disconnected process recovers missed writes via seq watermark catch-up): **PASS** — `TestCatchUpRecoversMissedEvents`, `TestListenerReconnects` (kills listener backend via `pg_terminate_backend`, asserts a later write still propagates), `TestMalformedNotificationTriggersCatchUp`.
- AC3 (single-process behavior + conformance suite unchanged): **PASS** — full `storetest` conformance + fuzz green; `just ci` exit 0; no `//go:build !race` tags.
- AC4 (no schema migration beyond TKT-M8400): **PASS** — feed reconciles from existing `seq`/`updated_at` columns; no new migration files. Cross-process SSE parity for entity create/update/delete verified by `TestStoreEventBridgeCrossProcessSSE`; relations/attachments explicitly out of the live feed (RR-GNS360 choice a).
- Resilience: `TestSelfNotificationFiltered` (no self-echo double-emit), `TestChannelIsolationAcrossSchemas` (per-schema channel), `TestListenerGoroutineExitsOnClose` (goleak, no goroutine/connection leak).

## Documentation (enhancements only)

- [x] User-facing documentation updated — `docs-project/entities/guides/GUIDE-postgres-backend.md` "Multiple writers" section (operator-focused after crit); CLAUDE.md multi-writer change-feed bullet; pgstore package doc.
- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: guide + project-doc update covered inline; no separate docs-checklist warranted for this scope)
- [x] ~~Docs-checklist marked as done~~ (N/A: see above)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (Test, Postgres Backend, Lint, Architecture, Fuzz, Frontend, Vulnerability Check, CodeQL all green; Rela Tickets passes after this checklist is marked done)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/898
