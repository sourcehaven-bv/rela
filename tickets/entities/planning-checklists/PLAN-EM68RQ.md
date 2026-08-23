---
id: PLAN-EM68RQ
type: planning-checklist
title: 'Planning: Wire the SQLite backend: sqlite build tag, appbuild recipe, release and CI isolation'
status: done
---

<!-- @managed: claude-workflow v1 -->

Second of the two stacked PRs for the SQLite backend. TKT-G91TBK built and
proved the store; this makes it selectable.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** the ticket's, unchanged. Build tag, appbuild recipe, the `rela db`
seam, GoReleaser entries, CI isolation assertions, docs.

OUT and worth restating: making SQLite the `rela-desktop` default. That is a
product call to make once the backend has been exercised, not a consequence of
it existing.

**Acceptance Criteria:** the ticket's seven; each maps to a check below.

## Research

- [x] Checked codebase for similar patterns or reusable code
- [x] Reviewed relevant rela concepts for prior art
- [x] ~~/research~~ (N/A: RES-03TUXO covers the design; this is its wiring)
- [x] ~~External libraries~~ (N/A)
- [x] ~~Reference implementations~~ (N/A — the postgres build IS the reference)

**Existing Solutions:** the `postgres` tag is the model, and it is a good one:
`appbuild_postgres.go` is a recipe over the shared `prepare`/`assemble` helpers,
so a new backend supplies `New` + `openBackend` and inherits everything
build-agnostic. `appbuild_fs.go` is the closer model for *this* backend, since
both pair a store with a bleve index wired as a write observer.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`appbuild_sqlite.go` provides exactly two symbols — `New` and `openBackend` —
and nothing else, matching every other recipe. The store opens at
`<CacheDir>/rela.db` (i.e. `.rela/rela.db`), which is where node-local runtime
state already lives; the bleve index is created first and installed via
`sqlitestore.WithObserver` so it receives writes from the start, then backfilled
with `backfillBleve`, exactly as the fs recipe does.

**The existing tags must widen, not just gain a sibling.** `appbuild_fs.go` is
`!postgres && !memorybackend`; without adding `&& !sqlite` the two recipes both
compile under `-tags sqlite` and `New` is declared twice. Same for
`internal/cli/db_nonpostgres.go` (`!postgres`), which additionally returns a
message naming the *filesystem* backend — wrong on both counts for sqlite.

**The three `!postgres` no-ops are inherited deliberately**, not by omission.
The SQLite backend enforces single-process at `Open` via the sidecar lock, so
the assumption each of them encodes is *true* here — but each gets a comment
saying so, because the default they inherit was written for fsstore and the
reasoning does not transfer automatically. Recorded in the ticket's table.

Alternatives rejected:

- **A fourth `*_sqlite.go` variant for each no-op.** Rejected: the behavior is
identical to the `!postgres` default, so three near-empty files would assert a
difference that does not exist. A comment at the inherited site is the honest
form.
- **Making SQLite the desktop default now.** Out of scope, and premature —
`rela-desktop` builds the default recipe today and switching it is a product
decision with its own migration question.

**Files to modify:**

- `internal/appbuild/appbuild_sqlite.go` (new), `appbuild_fs.go` (widen tag)
- `internal/cli/db_sqlite.go` (new), `db_nonpostgres.go` (widen tag + reword)
- `internal/appbuild/backendtest/backendtest_local.go` (widen tag if needed)
- `.goreleaser.yaml`, `.github/workflows/ci.yml`, `justfile`
- `docs/sqlite-backend.md` (new), `CLAUDE.md`, `README.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the database path is derived from
`cfg.Paths.CacheDir`, never from user input or a flag. That is deliberate and
mirrors the DSN rule: `RELA_DATABASE_URL` is env-only so a credential never
reaches `ps` or shell history. SQLite has no credential, but a caller-controlled
path would be a filesystem write primitive, so it stays derived.

**Security-Sensitive Operations:**

- **No new credential surface.** SQLite needs no DSN, so the sqlite build
simply does not have the class of concern the postgres build manages.
- **ACL is untouched.** Read gating lives in `internal/visibility` decorators
applied by `assemble`, which this recipe shares with every other build — the
recipe chooses a store, it does not gate reads (DEC-ZBI39P).
- **Config stays on disk.** `schema.yaml`, `acl.yaml`, `templates/`, `scripts/`
are read from the filesystem exactly as on the postgres build; SQLite backs
entities/relations/attachments only. A `--project` dir is still required.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Check | Pass condition |
|----|-------|----------------|
| 1 | `go build -tags sqlite ./...` | compiles; `just build-check-tags` covers it |
| 2 | cross-compile the six targets at `CGO_ENABLED=0` | all six |
| 3 | `rela db` under `-tags sqlite` | not the PostgreSQL message |
| 4 | `go list -deps` | default/postgres do not link `modernc.org/sqlite`; sqlite does not link pgx |
| 5 | `go test -tags sqlite ./internal/appbuild/...` | the existing wiring suite passes against the new recipe |
| 6 | read the three inherited no-ops | each carries a justification |
| 7 | arch-lint, lint, coverage | clean |

AC-5 is the real integration test: `appbuild`'s existing suite exercises the
assembled `Services`, so running it under the new tag proves the recipe wires a
working bundle rather than merely compiling.

**Edge Cases:** two recipes compiling at once if `appbuild_fs.go`'s tag is not
widened (a duplicate-`New` build failure, loud); `rela db` inheriting the wrong
message; the sqlite build accidentally linking bleve *and* being fine (it should
— unlike postgres, this backend uses bleve deliberately).

**Negative Tests:** `go list -deps` assertions are the negative tests — they
fail if a backend leaks into a build that must not carry it.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (m)

**Risks:**

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Forgetting to widen an existing tag, so two recipes compile | **High** | Fails loudly at build; `build-check-tags` runs all four tag combinations |
| Silently inheriting a `!postgres` no-op whose reasoning does not transfer | Medium | AC-6 requires a justification comment at each of the three sites |
| Release matrix grows without the isolation being pinned | Medium | AC-4 adds `go list -deps` assertions next to the existing pgx/bleve ones |
| Scope creep into making SQLite the desktop default | Medium | Explicitly out; `rela-desktop` untouched by this ticket |

**Effort:** m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** this is where the backend becomes user-visible, so the
docs deferred from TKT-G91TBK land here:

- `docs/sqlite-backend.md` — paralleling `postgres-backend.md`. Must state the
single-writer constraint, the WAL/network-filesystem limitation (**assumed, not
measured** — no network storage was available), and that content versioning is
not yet implemented, so this backend has neither git history nor version
history.
- `CLAUDE.md` — a row in the storage-backend table.
- `README.md` — backend selection guidance.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the design was settled in RES-03TUXO/DEC-LFSYNY and
reviewed there. Two findings from this arc's earlier reviews are carried in: the
`cli/db_nonpostgres.go` tag trap (found while surveying pgstore) and the three
`!postgres` no-ops (found in RES-03TUXO, and the reason single-process is
enforced rather than assumed). A fresh review runs against the finished diff.
