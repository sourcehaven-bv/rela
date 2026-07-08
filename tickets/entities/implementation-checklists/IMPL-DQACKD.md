---
id: IMPL-DQACKD
type: implementation-checklist
title: 'Implementation: pgstore content versioning: time-machine history + diff with principal attribution'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Implementation progress (vertical slices, test-first; DB-verified against real Postgres 15)

- [x] **Slice 1 — metamodel render-projection + content hash** (`internal/metamodel/projection.go`). GREEN.
- [x] **Slice 2 — migration `0004_versions.sql`** (schema_versions, entity_versions, version_seq, entities(updated_at)). DB-verified. Found+filed BUG-TY2XQC (pre-existing double-0003).
- [x] **Slice 3 — store DTOs + HistoryReader/VersionWriter** + pgstore `version.go` (list/get + vseq-fenced recursive-CTE lineage walk, WriteVersion in a tx). DB-verified.
- [x] **Slice 4 — synchronous rename/delete capture** (`entitymanager/version_hook.go`, VersionRecorder consumer-side dep, appbuild adapter). GREEN.
- [x] **Slice 5 — sweep goroutine** (`sweep.go`, one-connection advisory lock, settle+ceiling filter, batch cap, lifecycle-scoped dedup). DB-verified.
- [x] **Slice 6 — CLI** (`history.go`, `restore.go`). GREEN.
- [ ] **Slice 7 — data-entry** (history handlers via forWire/RR-YDMJV7, restore _action via validateFieldWrite/RR-VOYXRV, indistinguishable-404/RR-KDXGYK, `history:read` ACL). **NOT STARTED.**
- [ ] **Slice 8 — frontend** (HistoryPanel.vue + diff + client + mount). **NOT STARTED.**
- [ ] **Slice 9 — docs** (postgres-backend / cli-reference / acl-security / postgres CLAUDE.md). **NOT STARTED.**

## Reviews (cranky-code-reviewer + go-architect) — DONE on core + CLI

- **go-architect: structure approved** — all findings minor/nit, nothing blocks merge. Dependency direction, interface placement (both directions of consumer-side seam), optional-capability type-assert, goroutine ownership, no-storage-type-leak all satisfied. Noted `projectionHasher` duplicates canonical's framing (acceptable per "little copying" until a 3rd copy).
- **cranky-code-reviewer: found 2 CRITICAL wrong-history bugs** (both real, both mine, now fixed + regression-tested):
  - RR-7ZBISE reused-id merge → vseq-fenced recursive CTE.
  - RR-9O9RFZ delete-then-recreate dedup → lifecycle-scoped dedup.
  - RR-D0L7L0 (significant bundle) advisory-unlock-on-cancel, non-atomic capture, panic-on-write-path, µs interval, dead var, restore TOCTOU message, silent-noop log.
- All addressed; `golangci-lint` + `just arch-lint` clean; full non-DB suite + pgstore DB suite green.

## Verification (core + CLI)

- `go build ./...` and `-tags postgres ./...` clean.
- golangci-lint clean; arch-lint clean.
- Full non-DB suite passes.
- **pgstore DB suite green against real PostgreSQL 15** — concurrency + the two critical bug regressions exercised for real.

## Remaining (data-entry + frontend + docs)

Slices 7-9 add the *web* surface onto the same verified core. The CLI is a
complete, DB-verified end-to-end vertical, so the feature works today via CLI.
Data-entry is additive (a second surface + the field-ACL/redaction wiring the
security review mandated).
