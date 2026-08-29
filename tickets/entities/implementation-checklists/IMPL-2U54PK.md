---
id: IMPL-2U54PK
type: implementation-checklist
title: 'Implementation: Wire the SQLite backend: sqlite build tag, appbuild recipe, release and CI isolation'
status: done
---

<!-- @managed: claude-workflow v1 -->

Branch `feat/sqlite-wiring-TKT-L1A3PH`, stacked on the TKT-G91TBK branch.

## Development

- [x] Unit tests written for new code — the recipe is exercised by the
existing appbuild suite under the new tag rather than by new bespoke tests
- [x] Integration tests written — `go test -tags sqlite ./internal/appbuild/...`
runs the whole wiring suite against the new recipe. That is the real integration
proof: it assembles a working `Services` bundle, not just a compiling one.
- [x] Happy path implemented
- [x] Edge cases from planning handled — both existing tags widened; the three
inherited no-ops documented
- [x] Error handling in place — `Open`'s errors surface unchanged, because the
lock-held and WAL-unavailable messages are the actionable ones and wrapping them
in "open store" would bury the part an operator needs

## Test Quality

- [x] Using fixture builders or factories for test data — the appbuild suite's
own fixtures
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Verdict | Evidence |
|----|---------|----------|
| 1 — `-tags sqlite` builds | **PASS** | `just build-check-tags` now covers all four combinations |
| 2 — six targets at `CGO_ENABLED=0` | **PASS** | linux/darwin/windows × amd64/arm64, all six |
| 3 — `rela db` correct under sqlite | **PASS** | ran the built binary: reports "applied at open", never mentions PostgreSQL |
| 4 — CI isolation | **PASS** | five new assertions, all verified locally |
| 5 — appbuild suite under the tag | **PASS** | `appbuild` 1.9s · `appbuildtest` 0.6s · `backendtest` 1.5s |
| 6 — no-ops justified | **PASS** | all three carry a comment naming the single-process assumption and why it holds here |
| 7 — arch-lint / lint / coverage | **PASS** | arch-lint clean, `golangci-lint` 0 issues under both tags |

**Ran the real binary against a real project**, not just the test suite:

```
$ rela-sqlite --project /tmp/sqliteproj create feature -P title="Login flow"
✓ Created feature FEAT-D133
$ rela-sqlite --project /tmp/sqliteproj list feature
 FEAT-D133  feature  Login flow   open
 FEAT-JZEO  feature  Logout flow  done
$ sqlite3 .rela/rela.db "SELECT id, json_extract(properties,'$.title') FROM entities"
FEAT-D133|Login flow
FEAT-JZEO|Logout flow
$ ls entities/ | wc -l
0
```

Entity creation, ID generation, listing, relations and the data-migration
bootstrap all work; the data is genuinely in SQLite with **zero markdown
files**.

**The lock was verified with a real second process**, not a unit test — held the
sidecar `flock` from a separate Python process and ran the CLI:

```
sqlitestore: another process is using /tmp/sqliteproj/.rela/rela.db; this
backend is single-process by design (see DEC-LFSYNY). Stop the other
process, or use the PostgreSQL build for a multi-process deployment
```

and it worked again once the holder exited. That is the property the whole
single-writer design exists for, so proving it through the shipped binary rather
than through the package's own test mattered.

**One lint finding, fixed properly rather than suppressed.** `contextcheck`
caught that `sqlitestore.Open` discarded the caller's context. Added
`OpenContext` and threaded the recipe's ctx through, with a doc note that the
context bounds *opening only* — a caller passing a request context and expecting
it to cancel later writes would be wrong, so the two entry points are visibly
distinct.

## Quality

- [x] Code follows project patterns — recipe supplies only `New` +
`openBackend`; everything build-agnostic stays in `prepare`/`assemble`
- [x] Checked for DRY opportunities — the bleve helpers moved to
`bleveindex_shared.go` once a second recipe needed them, per CLAUDE.md's rule
that a recipe owns only its backend choice. Deliberately did NOT create three
near-empty `*_sqlite.go` no-op variants: the behavior is identical to the
`!postgres` default, so separate files would assert a difference that does not
exist
- [x] No security issues introduced — the DB path is derived from
`cfg.Paths.CacheDir`, never caller-supplied; no credential surface at all
- [x] No silent failures — the two failure modes that matter (lock held, WAL
unavailable) surface unchanged from `Open` with actionable text
- [x] No debug code left behind
