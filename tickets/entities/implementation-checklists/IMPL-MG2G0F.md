---
id: IMPL-MG2G0F
type: implementation-checklist
title: 'Implementation: Add comments.Store.Get so a single-comment read stops pulling the whole thread'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Coverage lands in the SHARED conformance suite rather than per-backend tests, so
all four implementations are held to one contract and a fifth cannot drift.
`RunGetTests` (with `runGetFidelityTests` and `runGetScopingTests`) joins
`RunAll`; the byte-exact key case joins `RunKeyFidelityTests`, which only the
database backends run — filecomments cannot honor it on a case-insensitive
filesystem.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Reuses the suite's existing `comment()` / `target()` builders and the `seed`
helper in `RunKeyFidelityTests`. The agreement test compares each `Get` result
against the `List` element rather than a literal, so it cannot drift from what
the backend actually stores.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Run against a live PostgreSQL 18 and an on-disk SQLite, both under `-race`.

- **AC1** — compiler. Adding `Get` to the interface broke all four backends plus
every test factory until each was implemented.
- **AC2** — EXPLAIN, not inspection. PostgreSQL on 20k seeded rows:
`Index Scan using comments_pkey`, `Buffers: shared hit=3`. The `List` it
replaces on that same 100-comment thread: `Bitmap Heap Scan` + `Sort`, `Buffers:
shared hit=117`, `Sort Method: quicksort Memory: 69kB`. SQLite: `SEARCH comments
USING INDEX sqlite_autoindex_comments_1 (target_key=? AND id=?)`.
- **AC3** — `TestGet_DoesNotReadTheWholeThread` asserts 1 get and 0 lists.
- **AC4/AC5** — conformance, all four backends.

**Mutation testing.** Every new assertion was verified to fail against the bug
it claims to catch, then restored:

| Mutation | Caught by |
|---|---|
| `target.ID` instead of `target.Key()` (drops face scoping) | `does not resolve across faces` |
| `ErrNoRows` → `nil` instead of `ErrNotFound` | 4 cases incl. both absent-id tests |
| `WHERE id = ?` only (ignores the target) | all 5 scoping cases |
| `Service.Get` reverted to list-and-scan | `TestGet_DoesNotReadTheWholeThread` |

**Driver behaviour verified empirically, not from docs.** The concern was a
deferred query error being misread as "no rows" and so reported as a 404 on an
infrastructure failure. Probed both drivers directly: a query against a missing
table surfaces at `Scan` as itself (`errors.Is(err, ErrNoRows) == false`) on
`modernc.org/sqlite` and `pgx`, while a genuinely empty result is `ErrNoRows`.
The `ErrNoRows` check precedes the generic wrap, so the fail direction is a 500,
not a phantom 404.

**A pre-existing divergence surfaced.** An assertion that an edited comment
carries an edit time passed on both database backends and failed on
filecomments/memcomments, which never set `UpdatedAt`. Out of scope here; filed
as TKT-JZY2PM and the suite pins only what `Get` owns — that its two read paths
agree — rather than papering over it.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

DRY: `sqlitecomments.scanComment` was widened from `*sql.Rows` to a `rowScanner`
interface so the thread read and the single-row read share one decoder — the
alternative was a second copy of the timestamp parsing and anchor decoding to
keep in step. Named `rowScanner` rather than `scanner` because `sql.Scanner` is
a different interface and this file imports `database/sql`.

Security: `Get` is on the authorization path (its result decides an `*-own`
permission), so scoping is asserted in both directions plus the same-id-on-two-
faces case that distinguishes "returns the right row" from "returns a row".
Arguments are bound, never interpolated. `just arch-lint` confirms neither
database backend gained a dependency on `internal/store`.
