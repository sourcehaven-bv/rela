---
id: IMPL-DQACKD
type: implementation-checklist
title: 'Implementation: pgstore content versioning: time-machine history + diff with principal attribution'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Implementation progress (vertical slices, test-first)

Building bottom-up. Each slice: test → code → green → commit.

- [x] **Slice 1 — metamodel render-projection + content hash** (`internal/metamodel/projection.go` + test). `RenderProjection()` extracts render-relevant schema (property defs, display_property, property order, enum values); `Hash()` content-addresses it with length-prefixed SHA-256 (mirrors `internal/canonical`). Tests: determinism/map-order stability, stable-across-churny-edits (automations/validations/colors/descriptions/defaults), changes-on-render-relevant-edits (type/required/list/format/enum/order/display). GREEN. Commit c7c06ed6.
- [ ] **Slice 2 — pgstore migration `0004_versions.sql`**: `schema_versions`, `entity_versions`, `version_seq`, `entities(updated_at)` index. + migrate test.
- [ ] **Slice 3 — store DTOs + `store.HistoryReader` interface** (`store.go`) + pgstore `version.go` writer/reader; conformance where applicable.
- [ ] **Slice 4 — synchronous capture**: rename (op=rename + prev_id at manager.go:675) + delete (BEFORE Store.DeleteEntity, ~manager.go:598) via consumer-side VersionRecorder dep; no-op recorder for fs/mem.
- [ ] **Slice 5 — sweep goroutine** (`sweep.go`): same-connection `pg_try_advisory_lock`, settled+ceiling filter, batch cap, DISTINCT ON/LATERAL probe, dedup; lifecycle start/stop (open.go/Close); wired postgres-only.
- [ ] **Slice 6 — read/render**: HistoryReader wiring, render-only-metamodel-from-projection, CLI `rela history` (+snapshot output) + `rela restore` (three-file build split).
- [ ] **Slice 7 — data-entry**: history handlers via `forWire` (RR-YDMJV7), restore `_action` via `validateFieldWrite` (RR-VOYXRV), indistinguishable-404 (RR-KDXGYK); `history:read` ACL gate.
- [ ] **Slice 8 — frontend**: `HistoryPanel.vue` + diff view + client + mount.
- [ ] **Slice 9 — docs**: postgres-backend / cli-reference / data-entry / acl-security / postgres CLAUDE.md.

## Development

- [x] Unit tests written for new code (slice 1)
- [ ] Integration tests written (DB-gated pgstore tests — slices 2-5)
- [x] Happy path implemented (slice 1)
- [ ] Edge cases from planning handled (in progress)
- [x] Error handling in place (slice 1: hasher panics on unreachable; no swallowing)

## Test Quality

- [x] Using fixture builders (baseMetamodel() in slice 1)
- [x] No hardcoded values where object in scope
- [x] Only specifying values that matter
- [x] Interpolated values from objects
- [x] Property comparisons use original object

## Manual Verification

- [ ] Feature manually tested end-to-end (pending surfaces — slices 6-8)
- [ ] Each acceptance criterion verified
- [ ] Edge cases manually verified

**Verification Evidence:**
- Slice 1: `go test ./internal/metamodel/` green; projection hash stable across churny edits and drift-sensitive to render-relevant edits (the dedup-correctness property the design relies on).

## Quality

- [x] Code follows project patterns (mirrors internal/canonical writer discipline)
- [x] Checked for DRY (reused package `sortedKeys`; hasher intentionally parallels canonical's writer — not extracted since canonical's is entity-specific + unexported)
- [x] No security issues introduced (slice 1)
- [x] No silent failures
- [x] No debug code left behind
