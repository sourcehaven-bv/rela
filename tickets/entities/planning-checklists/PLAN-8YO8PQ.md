---
id: PLAN-8YO8PQ
type: planning-checklist
title: 'Planning: SQLite store backend: conformance-passing minimal store behind a sqlite build tag'
status: done
---

<!-- @managed: claude-workflow v1 -->

Split into two stacked PRs (operator decision): **PR 1 = the store** (#1421),
**PR 2 = the wiring**. Each is reviewable on its own, and the store is provable
before anything depends on it.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

PR 1 (this one): `internal/store/sqlitestore` — the 26 `store.Store` methods,
the single-writer lock, the WAL guard, and the archfile component.

PR 2: the `sqlite` build tag, `appbuild_sqlite.go`, `cli/db_sqlite.go`,
GoReleaser entries, CI isolation assertions, `justfile` build-check-tags.

OUT of the ticket entirely: versioning, `StateKV`, `UserState`, native FTS5
search, SQL pushdown, `DerivedSchemaReconciler`, `ManifestSince`, and
multi-process anything.

**Acceptance Criteria:** the ticket's eight, of which 1-4 and 6 belong to PR 1
and 7 to PR 2. Each maps to a test named below.

## Research

- [x] For larger features: run `/research` — RES-03TUXO
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-03TUXO → DEC-LFSYNY (accepted). The spike branch
`spike/sqlite-tx-TKT-TWIO11` is the measured reference implementation.

**Existing Solutions:**

- **pgstore** is the closest reference — the other SQL backend. Tx shape,
keyset pagination, case-identity index and JSON normalization are all adapted
from it.
- **fsstore** for `graphquerynaive` delegation (28 lines) and observer fan-out.
- **memstore** for `HighestID`'s `prefix + "-"` convention and the rename
event choice.
- **storeutil** for cursors, ID/relation validation and `TopValues`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** port the spike's proven core (Tx, entity/relation CRUD,
events), then implement what it deliberately stubbed — attachments, pagination,
`RenameEntity`, `HighestID`, `PropertyValues`, observers.

Three decisions worth recording:

1. **Take the strong Tx tier.** The spike measured that SQLite gives rollback
and post-commit-only events essentially free, so offering the reduced fs/mem
contract would be a choice to be worse.
2. **Enforce single-process at `Open`, don't document it.** `unique:` is an
untransacted entitymanager scan; with two processes there is no backstop and the
failure is silent. A sidecar `flock` turns that into a refusal.
3. **Delegate `GraphQuery` to `graphquerynaive`.** It is the behavioral
reference every backend is verified against, so this backend agrees with the
others from day one. Pushdown is a later optimization that must reuse
`DepthCap`.

Alternatives rejected: locking the database file itself (SQLite holds POSIX
locks on that inode and any fd close drops them); `MaxOpenConns(1)` to serialize
writers (measured to deadlock on `database/sql` starvation); building a
`'$.<name>'` JSON path (escaping a second grammar inside SQL — the fuzz suite
found the failure).

**Files to modify:** `internal/store/sqlitestore/*.go` (new package),
`.go-arch-lint.yml` (component + vendor), `go.mod`/`go.sum`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Entity IDs and relation types** → `storeutil.ValidateID` /
`ValidateRelationType` on every write path. The fuzz suite treats these as the
validity ORACLE: anything storeutil rejects, the store must reject.
- **Attachment file names** → `store.ValidateFileName`, plus
`CapAttachmentReader` at `MaxAttachmentBytes` as the mandatory backstop.
- **Property names** → `storeutil.ValidateProperty`, and passed as a **bound
parameter** to `json_each`. The first implementation concatenated the name into
a JSON path string; the fuzz suite found that a `"` in the name breaks it.
Escaping for a second grammar nested inside SQL is the shape that goes wrong
quietly, so the query was restructured to avoid needing to.
- **All SQL is parameterized.** No user value is ever concatenated into a
statement.

**Security-Sensitive Operations:**

- **The database path** is derived from the caller's project config, never
from request input.
- **No credential handling at all** — SQLite has no DSN or password, which
removes the `RELA_DATABASE_URL` class of concern rather than relocating it.
- **ACL is unaffected**: read gating lives in `internal/visibility` decorators
at the wiring site, never in a store (DEC-ZBI39P). This backend adds no
redaction and must not.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Pass condition |
|----|------|----------------|
| 1 | `storetest.RunAll` | passes with `{Attachments: true, TxRollback: true}` |
| 2 | `RunTxStressTest`, `RELA_STRESS_SECONDS=30` | no watchdog dump, no lost updates, pair atomicity |
| 3 | `RunTxRollbackTests` (via `TxRollback`) | rollback + no events pre-commit |
| 4 | six fuzz targets | no crashes, no invariant violations |
| 5 | `TestSecondOpenIsRefused` | second `Open` fails, error names the holding pid |
| 6 | `TestWALIsEnabled` | `journal_mode == "wal"` |

The conformance suite IS the integration test: it exercises the real backend
through the same contract every other store is held to.

**Edge Cases:** nested `Tx` joining without a pool acquire; ctx cancellation
mid-Tx (rollback via `WithoutCancel`); empty store `LastModified`; case-variant
IDs; JSON numbers, nested maps and arrays; property names containing quotes;
attachment cascade on delete; lock release on `Close` (refusing forever would be
worse than not locking).

**Negative Tests:** second opener refused; oversize attachment rejected without
touching an existing row; invalid cursor rejected; empty relation type rejected.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (l)

**Risks:**

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Silent divergence from the other backends | **High** (materialized: 7 real bugs) | The conformance suite, run early and often. Every bug listed in the PR was caught by it, not by review |
| Reintroducing a spike finding by accident | Medium | All three are in the package doc with their measurements; `verifyBusyTimeout` fails at `Open` rather than under load |
| Single-process assumed rather than enforced | Medium | `flock` at `Open` + `TestSecondOpenIsRefused` |
| WAL silently unavailable on a sync filesystem | Medium | Verified at `Open` and refused, with an error naming the likely cause |
| Scope creep into versioning/state | Medium | Archfile `mayDependOn` is deliberately narrower than pgstore's, so widening it requires justification |

**Effort:** l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** deferred to PR 2, where the backend becomes selectable
and therefore user-visible:

- `docs/sqlite-backend.md` paralleling `postgres-backend.md` — the
single-writer constraint, the WAL/network-filesystem limitation, and the
git-versus-version-history trade-off
- `CLAUDE.md` — the storage-backend table gains a row
- `README.md` — backend selection guidance

Nothing in PR 1 is reachable by a user, so there is nothing to document yet.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the design was reviewed as part of TKT-TWIO11 (4
critical + 4 significant, all addressed there), and its two most valuable
findings are carried into this implementation: the sidecar-not-database-file
lock, and the fact that `MaxOpenConns(1)` deadlocks rather than serializes. A
fresh review pass runs against the finished code in the review phase.
