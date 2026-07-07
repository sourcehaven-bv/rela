---
id: IMPL-DQACKD
type: implementation-checklist
title: 'Implementation: pgstore content versioning: time-machine history + diff with principal attribution'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Implementation progress (vertical slices, test-first; DB-verified against real Postgres)

- [x] **Slice 1 — metamodel render-projection + content hash** (`internal/metamodel/projection.go`). RenderProjection + Hash (length-prefixed SHA-256) + JSON. Tests: determinism, stable-across-churny-edits, drift-sensitive. GREEN.
- [x] **Slice 2 — migration `0004_versions.sql`**: schema_versions, entity_versions, version_seq (NOT rela_seq — RR-E5AH72), entities(updated_at) index. migrate-integrity test (grandfathers pre-existing BUG-TY2XQC double-0003). DB-verified.
- [x] **Slice 3 — store DTOs + HistoryReader/VersionWriter** (`store.go`) + pgstore `version.go`: list/get with rename-lineage walk (op=rename/prev_id), WriteVersion, dedup helpers. DB-verified (write/list/get, rename lineage, ordinals, ErrNotFound).
- [x] **Slice 4 — synchronous capture** (`entitymanager/version_hook.go`): VersionRecorder consumer-side dep; rename (op=rename+prev_id at manager.go) + delete (BEFORE Store.DeleteEntity — RR-Q79T54). appbuild adapter wires it when store supports VersionWriter (nil for fs/mem). Tests: delete/rename capture + attribution + nil-recorder no-op. GREEN.
- [x] **Slice 5 — sweep goroutine** (`sweep.go`): whole tick on ONE acquired connection under pg_try_advisory_lock (RR-FB6QU8); settled-OR-stale-ceiling filter (RR-J188VJ); batch cap + LATERAL probe (RR-HLDQ6H); content-hash dedup; create/update capture w/ system principal; lifecycle start/stop; build-tagged appbuild wiring. DB-verified (captures settled, debounces fresh, dedups). **Found + fixed via real-DB run: nullable content_hash scan; debounce semantics.**
- [x] **Slice 6 — CLI** (`history.go`, `restore.go`): timeline + `--version N` snapshot for external diff (Unix philosophy); restore = validated entitymanager write (update-if-live / recreate-if-deleted); three-file build split via kong + allowlist. Unsupported-backend degradation tests. GREEN.
- [ ] **Slice 7 — data-entry**: history read handlers via `forWire`/stripHiddenProperties (RR-YDMJV7); restore `_action` via validateFieldWrite (RR-VOYXRV); indistinguishable-404 (RR-KDXGYK); `history:read` ACL permission (RR-D8NWM4). **NOT STARTED.**
- [ ] **Slice 8 — frontend**: HistoryPanel.vue + diff view + client + mount. **NOT STARTED.**
- [ ] **Slice 9 — docs**: postgres-backend / cli-reference / acl-security / postgres CLAUDE.md. **NOT STARTED.**

## Verification status

- `go build ./...` and `go build -tags postgres ./...` both clean.
- `golangci-lint` clean on all changed packages; `just arch-lint` clean (added pgstore→canonical).
- Full non-DB suite passes.
- **pgstore DB suite passes against a real PostgreSQL 15** (conformance + new version tests) — the concurrency/atomicity design was exercised for real, not just compiled.
- Found BUG-TY2XQC (pre-existing double-0003 migration prefix) while building; filed, grandfathered in the new integrity test.

## Review

- [ ] cranky-code-reviewer + go-architect on the core + CLI (running now, before data-entry).

## Quality (core + CLI)

- [x] Unit + DB-integration tests written and passing
- [x] Follows project patterns (canonical writer discipline, Formatter capability pattern, rela db build-split, listener lifecycle model, tombstone intra-tx precedent)
- [x] No security issues in the core paths (attribution from ctx only; store never learns Principal by other route)
- [x] No silent failures (sweep logs+continues per-entity; version write best-effort by design, logged)
- [x] No debug code left behind
