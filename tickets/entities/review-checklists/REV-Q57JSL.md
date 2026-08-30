---
id: REV-Q57JSL
type: review-checklist
title: 'Review: Promote ProjectionProvider, SweepConfig and VersionStore into internal/store so sweep capabilities are backend-neutral'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`) — clean under default and `--build-tags postgres` for the touched packages; one pre-existing `govet: shadow` in `derivedschema_postgres.go:55` is untouched by this diff
- [x] Comment lint gate clean (`just comment-lint`) — no unresolvable doc links across 11445 comments; `comment-report` shows no advisory finding introduced by this diff
- [x] Coverage maintained (`just coverage-check`) — 78.5% total, package and total thresholds PASS

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (RR-BJDCMI, RR-P8HARP)
- [x] All significant review-responses addressed (RR-CNY4SZ, RR-UZ83ET, RR-D3OH9M)
- [x] Self-reviewed the diff for unrelated changes — 9 files, all in the capability-widening path; no TODO/FIXME introduced

**Review Responses:** RR-BJDCMI (critical), RR-P8HARP (critical),
RR-CNY4SZ (significant), RR-UZ83ET (significant), RR-D3OH9M (significant),
RR-B615WK (minor), RR-O2HRM3 (minor) — all `addressed`.

The two criticals are the finding of record: widening `VersionStore()` to an
interface **disabled** the typed-nil guard the same commit claimed to
strengthen, and both tests covering that guard were vacuous — one had silently
stopped satisfying the widened interface, the other used an untyped nil that
never enters the guard's real path. Full green over a broken guard. Fixed with
one shared `nonNilCapability` helper rather than three hand-rolled checks, and
both tests mutation-verified: with the helper deleted they fail, with it
present they pass.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **PASS** — no pgstore type appears in a capability interface or its return
   type. `rawStateStoreFor` still *calls* `pgstore.StateStoreFor` in its body,
   which is correct: that is the backend-specific discovery site, in a
   `_postgres.go` file, and its return is now an interface.
2. **PASS** — `backendneutral_postgres_test.go` builds doubles for every
   capability from store-package types alone and does not import pgstore. A
   pgstore type creeping back into a signature makes the file stop compiling.
3. **PASS** — exported method count on `pgstore.Store` unchanged (1 before, 1
   after in `version_store.go`); `just plimsoll` clean.
4. **PASS** — `go test -tags postgres -race` green for `internal/appbuild` and
   `internal/store/pgstore` against a real database; default-build store suites
   green (fsstore, memstore, pgstore, sqlitestore, storetest, storeutil).
5. **PASS** — `just arch-lint` OK; golangci-lint clean under both tag sets for
   the touched packages.

Additionally verified beyond the stated criteria: the alias decision is now
pinned by a compile-time assertion that was regression-tested — converting
`pgstore.SweepConfig` from an alias to a named type breaks the build at those
lines in both directions. And `go build -tags sqlite ./...` succeeds, which is
the downstream reason this ticket exists.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor, no user-facing surface)
- [x] ~~User-facing documentation updated~~ (N/A: internal refactor, no user-facing surface)
- [x] ~~Docs-checklist marked as done~~ (N/A: internal refactor, no user-facing surface)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — TKT-G91TBK is the consumer; the sqlite build compiles against the promoted types

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI — PR #1478

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
