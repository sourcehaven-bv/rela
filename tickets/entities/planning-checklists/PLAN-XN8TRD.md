---
id: PLAN-XN8TRD
type: planning-checklist
title: 'Planning: rela-desktop runs on the SQLite backend and opens self-contained projects'
started: "2026-10-04"
completed: "2026-10-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

Recorded after implementation: the work landed on feat/self-contained-sqlite
before this checklist was filled in. Each item states what was done.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: the desktop app on the sqlite build; `.rela` documents holding config, data,
history and settings; File menu export/import of config and data; keychain
secrets keyed by document ID; AI and mail settings in `state.KV`; audit no-op on
the desktop; FTS5 search in the database; native menus, downloads and
notifications (`desktop.yaml`).

Out: removing the per-document root folder (`project.Context` still needs a
root); ACL on the desktop; macOS host integration (Spotlight, App Intents).

**Acceptance Criteria:**

1. `just build-desktop` produces a sqlite build. Test: CI dependency checks;
`cmd/rela-desktop` sqlite tests.
2. A self-contained project opens from the welcome screen, recent list and
Finder. Test: `document_sqlite_test.go`, `database_sqlite_test.go`.
3. Export/import of config round-trips. Test: `database_sqlite_test.go`,
`appbuild` projectfiles tests.
4. Secrets survive a document move, and are released only at a trusted place.
Test: `hostconfig_test.go`, `settings_sqlite_test.go`.
5. Search finds the same entities as the postgres backend. Test: storetest
search conformance with the FTS5 backend; `sqlitestore/search_test.go`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: decisions recorded instead: DEC-LFSYNY, DEC-R9M57Z, DEC-10Z731)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows pgstore's in-DB search and state KV)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A; see DEC-10Z731 and
`docs/architecture/desktop-documents.md`.

**Existing Solutions:**

- `github.com/zalando/go-keyring` for OS credential stores (chosen: pure Go,
all three platforms; per-app access control would need cgo and signing).
- SQLite FTS5 `trigram` tokenizer (chosen: built in; modernc has no custom
tokenizer API). Bleve was rejected because its index lives outside the file.
- Mirrors pgstore: in-DB search backend, `state.KV` in the database.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** documented in `docs/architecture/desktop-documents.md`.
`lua.HostConfig` abstracts secrets and AI/mail settings;
`appbuild.WithHostConfig` lets the desktop replace the `.rela` files with
keychain plus `state.KV`. FTS5 index maintained by triggers inside each write
transaction.

**Files to modify:** `cmd/rela-desktop/*`, `internal/appbuild/*`,
`internal/sqlitedb/*`, `internal/store/sqlitestore/search.go`,
`internal/hostconfig/`, `internal/lua/`, `internal/ai/`, `internal/mail/`,
`internal/rootfs/`, docs and guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Document ID from the file: must match `^[0-9a-f]{32}$`, else replaced.
- Config files from the database: names allowlisted per area; disk reads
contained with `os.Root` (`internal/rootfs`).
- Search needle: bound as a parameter; quoted as an FTS5 phrase; LIKE
wildcards escaped.
- Window routes from the page: `safeRoute` / `safeBase`.

**Security-Sensitive Operations:**

- Keychain secrets: released only to a place the user trusted; values never
sent to the page.
- A document carries code: documented; config can be exported for review.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria above.

**Edge Cases:** needles shorter than a trigram; unicode case folding; quotes,
`%` and `_` in needles; rowid reuse after delete; world prime-face selection;
v10 database migration; malformed document IDs; place list bound.

**Negative Tests:** untrusted place gets no secrets and cannot set one; nil
constructors rejected; FIFO and escaping symlinks refused by rootfs.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A shared document could carry the ID of the user's own document and read its
secrets. Mitigated by per-place trust.
- FTS5 index grows the file. Accepted; documented.
- SQLite on a synced folder corrupts. Mitigated by the WAL check and docs.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-desktop (new), GUIDE-sqlite-backend,
`docs/architecture/desktop-documents.md`, `docs/sqlite-backend.md`, CLAUDE.md,
README.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design settled in DEC-LFSYNY, DEC-R9M57Z and DEC-10Z731; a security review during implementation found two keychain issues, both fixed)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** keychain trust by document ID alone, and implicit
trust on `set`: both fixed (per-place trust, `errNotTrusted`).
