---
id: IMPL-6YNM
type: implementation-checklist
title: 'Implementation: Detect git-crypt encrypted files at fsstore and show inaccessible placeholders in data-entry'
status: done
---

## Development

**Code changes:**

- [x] `internal/entity/entity.go` — added `InaccessibleField`, `InaccessibleReason` enum, `Inaccessible []InaccessibleField` on `Entity` and `Relation`, with `IsInaccessible(name)` helpers and Clone propagation.
- [x] `internal/store/fsstore/gitcrypt.go` (new) — `isGitCryptEncrypted` with the 10-byte magic-header check.
- [x] `internal/store/fsstore/gitcrypt_test.go` (new) — 13 table-driven cases covering exact-match, ciphertext suffix, conflict-marker bytes after header, near-misses, BOM, normal markdown, nil/empty.
- [x] `internal/store/fsstore/markdown.go` — `readEntityFile` and `readRelationFile` now take `(id, type)` / `(from, type, to)`, detect encryption via `isGitCryptEncrypted`, build a shell entity/relation via `buildInaccessibleEntity` / `buildInaccessibleRelation`. Detection happens BEFORE `parseDocument`, so the conflict-marker scan never sees encrypted bytes.
- [x] `internal/store/fsstore/propcache.go` — updated `loadEntity`/`loadRelation` callers.
- [x] `internal/store/fsstore/watcher.go` — `reconcileEntityPath` checks magic header before invoking `parseEntityFromPath`, and uses new `entityIdentityFromPath` helper to derive (id, type) from the file path. Closes the watcher gap (RR-CQCIR).
- [x] `internal/validator/validator.go` — `loadCandidates` skips entities with non-empty `Inaccessible` so property-driven rules don't produce false-positive "required field missing" violations.
- [x] `internal/dataentry/api_v1.go` — extended `V1Entity` with `Inaccessible []V1InaccessibleField`; `entityToV1` propagates the field; `handleV1UpdateEntity` rejects PATCH on inaccessible entity with HTTP 422 + `encrypted_inaccessible` error type before invoking the entity-manager write path.
- [x] `frontend/src/types/entity.ts` — extended `Entity` with `inaccessible?: InaccessibleField[]`.
- [x] `frontend/src/components/lists/EntityList.vue` — `isCellInaccessible(entity, column)` helper; renders 🔒 lock icon (with tooltip "Encrypted...") for inaccessible cells in both desktop table and mobile card layouts. Wildcard `*` matches any column.
- [x] `frontend/src/components/entity/EntityDetail.vue` — `isInaccessible` and `inaccessibleNames` computed; banner with "🔒 This entity is git-crypt encrypted..." + link to git-crypt README; Edit button hidden when inaccessible (both desktop + mobile); E keyboard shortcut early-returns; per-property lock indicator passed to PropertyDisplay via the new `inaccessible` flag on `PropertyItem`.
- [x] `frontend/src/components/common/PropertyDisplay.vue` — extended `PropertyItem` with `inaccessible?: boolean`; renders "🔒 encrypted" placeholder text for locked properties.

**Integration tests:**

- [x] `internal/store/fsstore/gitcrypt_integration_test.go` (new) — `GetEntity`, `ListEntities`, `GetRelation`, half-encrypted relation case (cleartext relation pointing at encrypted entity).
- [x] `internal/dataentry/inaccessible_test.go` (new) — `entityToV1` propagates `Inaccessible`; PATCH against inaccessible entity returns 422 with `encrypted_inaccessible` type URL.

## Manual verification

Manual end-to-end test against `/tmp/gitcrypt-test` fixture (a copy of the
data-entry prototype project) with one encrypted entity injected via `python3 -c
"import sys, os; sys.stdout.buffer.write(b'\\x00GITCRYPT\\x00' +
os.urandom(120))" > entities/tickets/TKT-LOCKED.md`:

| AC | Verified | Evidence |
|---|---|---|
| AC1 fsstore detects header | ✅ | `GetEntity(TKT-LOCKED)` returns Entity with empty Properties + 9 inaccessible fields. |
| AC2 list view 🔒 | ✅ | Screenshot `all-tickets-list` shows row of lock icons in title/status/priority/assignee/reporter/due cells for TKT-LOCKED. |
| AC3 detail view banner + locks | ✅ | Screenshot `locked-detail` shows "🔒 This entity is git-crypt encrypted" banner with code-formatted `git-crypt unlock` and external link, plus all 9 form fields rendered as "🔒 encrypted". |
| AC4 PATCH 4xx | ✅ | `curl -X PATCH ... TKT-LOCKED` → HTTP 422 with body `{"type":"https://rela.dev/errors/encrypted_inaccessible","title":"Cannot edit an inaccessible entity",...}`. |
| AC5 unit edge cases | ✅ | `gitcrypt_test.go` covers all 13 cases including conflict-marker collision regression. |
| AC6 integration tests | ✅ | `gitcrypt_integration_test.go` covers entity, relation, list, half-encrypted scenarios. |
| AC11 magic-header BEFORE parser | ✅ | Detection in `readEntityFile` happens before `parseDocument`, verified by test case "magic header followed by conflict-marker bytes". |
| AC13 watcher transition | ✅ | After replacing encrypted file with cleartext markdown, `GET /api/v1/tickets/TKT-LOCKED` returned full Properties with `inaccessible` array empty. After re-encrypting, `properties` returned `{}` with 9 inaccessible fields again. Bidirectional. |

**Bonus UX polish (added during implementation, beyond spec):**

- Edit button hidden on inaccessible entities (both desktop + mobile + keyboard shortcut). Prevents user from opening an empty edit form and getting a 422 on save.

## Quality

- [x] `go build ./...` clean
- [x] `go test -race ./...` — all packages pass including `internal/store/fsstore`, `internal/validator`, `internal/dataentry`, `internal/entity`
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — OK no warnings
- [x] `cd frontend && npm run typecheck` — clean
- [x] `cd frontend && npm run test:run` — 601 tests pass
- [x] `cd frontend && npm run lint` — 0 errors (74 pre-existing warnings, none new)
- [x] `cd frontend && npm run build` — clean

## Coverage delta

- New: `internal/store/fsstore/gitcrypt.go` + `gitcrypt_test.go` (13 cases)
- New: `internal/store/fsstore/gitcrypt_integration_test.go` (4 integration tests)
- New: `internal/dataentry/inaccessible_test.go` (3 tests)
- Modified: `internal/entity/entity.go` — Clone tested via existing entity_test.go (round-trip preserved)

No coverage floor reductions; all changes additive or covered.
