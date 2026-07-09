---
id: IMPL-DQACKD
type: implementation-checklist
title: 'Implementation: pgstore content versioning: time-machine history + diff with principal attribution'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Implementation — COMPLETE (all 9 slices; DB-verified against real Postgres 15; demoed in-browser)

- [x] **Slice 1** metamodel render-projection + content hash (`internal/metamodel/projection.go`)
- [x] **Slice 2** migration `0004_versions.sql` (schema_versions, entity_versions, version_seq, entities(updated_at)); found+filed BUG-TY2XQC
- [x] **Slice 3** store HistoryReader/VersionWriter + pgstore `version.go` (vseq-fenced recursive-CTE lineage)
- [x] **Slice 4** synchronous rename/delete capture (entitymanager VersionRecorder hook + appbuild adapter)
- [x] **Slice 5** reconciliation sweep goroutine (`sweep.go`, single-connection advisory lock, settle+ceiling, batch cap, dedup)
- [x] **Slice 6** CLI `rela history` / `rela restore`
- [x] **Slice 7** dataentry history read + restore endpoints (forWire redaction RR-YDMJV7, field-validated restore RR-VOYXRV, indistinguishable-404 RR-KDXGYK, `history:read` permission RR-D8NWM4)
- [x] **Slice 8** frontend HistoryPanel.vue + dependency-free diff + restore, mounted in EntityDetail (1166 FE tests pass)
- [x] **Slice 9** docs (postgres-backend, cli-reference, acl-security, CLAUDE.md)

## Verification (final)

- `go build ./...` + `-tags postgres ./...` clean.
- Full Go non-DB suite passes; **pgstore DB suite passes against real PostgreSQL 15** (conformance + versioning + the 2 critical-bug regressions).
- `golangci-lint` 0 issues on all changed packages; `just arch-lint` clean.
- Frontend: 1166 tests pass, `vue-tsc` 0 errors, production `vite build` succeeds.
- **End-to-end demo verified in-browser** (Puppeteer): built `rela-server-postgres`, seeded a ticket + 3-version history (alice/bob attribution), the Version-history panel renders the timeline + per-version diff + Restore buttons on the entity detail page.

## Reviews

- 2 design-review rounds + cranky-code-reviewer + go-architect (see linked review-responses). go-architect: structure approved (all minor/nit). cranky: 2 critical wrong-history bugs found + fixed with DB regressions. All slice-7 security findings (RR-YDMJV7/VOYXRV/KDXGYK) implemented and marked addressed.

## Follow-ups filed

TKT-N0OWKE (intra-tx atomicity), TKT-VFJKMB (relation history), IDEA-ADI72Q
(live-entity schema_hash), TKT-BW6UUL (purge-version), BUG-TY2XQC (double-0003
migration prefix), **TKT-ZIRMGM (author-aware capture: last_edited_by +
flush-on-author-change — user's follow-up idea)**.

## Manual verification evidence

Demo: `rela-server-postgres` on :8087, project /tmp/relademo, DB
rela_versiondemo. TKT-YSEM has v1 create (alice) / v2 update (bob) / v3 update
(alice); GET /api/v1/_history/ticket/TKT-YSEM returns the timeline with
attribution; the SPA panel shows it with a working diff.

## Note on branch state

All work is on branch `feat/pgstore-versioning-TKT-9INY0Y` (11+ commits), not
merged/pushed. Next: `/pr`.
