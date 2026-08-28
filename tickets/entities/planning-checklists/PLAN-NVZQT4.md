---
id: PLAN-NVZQT4
type: planning-checklist
title: 'Planning: Reachability floor: merged-coverage pipeline + scupper (report-only baseline)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** `.testcoverage.yml` enforces per-package coverage *percentages* from
the standard unit-test profile. That measures how well a package is tested, but
it cannot answer a weaker and more basic question: has this line run **at all**,
under *any* test? A package can sit above its threshold while still containing
statements no test has ever executed — dead branches, unreachable-in-practice
paths — unobserved until a user hits them in production.

**Scope (in):**
- `scripts/reachability.sh`: collect coverage from unit + cross-package
  (`-coverpkg`), postgres-tagged, and e2e sources as Go binary coverage, merge
  with `go tool covdata`, report via `scupper`.
- `just reachability` recipe.
- A **non-blocking, report-only** CI job.
- `scupper` pinned at `v0.2.0`, invoked `-d coverage-ignore --require-reason`.
- rela-server graceful shutdown (prerequisite — see Approach).
- e2e `-cover` wiring (`RELA_E2E_COVERDIR`) in the Playwright fixtures.

**Scope (out):**
- Any enforced threshold or rising per-package floor. This pass is report-only
  on purpose: a threshold picked before the baseline is known is arbitrary.
- Annotating the full remaining set of never-executed blocks.
- Replacing `.testcoverage.yml`. The two gates coexist and measure different
  things.

**Acceptance Criteria:**
1. `scripts/reachability.sh` merges unit + cross-package coverage and reports a
   reachability number; postgres and e2e legs activate on
   `RELA_TEST_DATABASE_URL` / `RUN_E2E=1`.
2. CI job is report-only: does not fail the build, not a required check.
3. Every dismissal carries a reason (`--require-reason` passes).
4. rela-server shuts down gracefully on SIGTERM/SIGINT so coverage is flushed.
5. `go build ./...` and `go test ./...` pass.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the tool survey is short enough to record here.

**Existing Solutions:**
- **`vladopajic/go-test-coverage`** — already in the repo via `.testcoverage.yml`.
  Does per-package percentage floors from a single profile. It does not merge
  multiple coverage sources, which is exactly what a reachability floor needs.
  Kept; not replaced.
- **`sourcehaven-bv/scupper`** — reads a merged profile, drops explicitly
  dismissed blocks, reports/enforces reachability. Chosen because it is the only
  piece that does the dismissal-aware filtering; the merging itself is stock
  `go tool covdata`.
- **Prior art for the directive idea**: PHP's `@codeCoverageIgnore`, Python's
  `# pragma: no cover`. Go's toolchain has no equivalent, which is the gap.

**Directive-syntax finding (decides a real fork in the road).** scupper's
directive keyword is configurable via `-d`, so *two* spellings are functional:
its own default `//scupper:ignore` and the go-test-coverage style
`// coverage-ignore:`. Verified against the pinned v0.2.0 by building the tool
and running it over a fixture containing both spellings: each run dismissed the
spelling matching its `-d` and reported the other as unreached. Neither is
obsolete — the choice is ours.

Chose **`coverage-ignore`** because `.testcoverage.yml` already sets
`force-annotation-comment: true` and understands that spelling. One annotation
vocabulary serves both tools; `//scupper:ignore` would be a second dialect
meaning the same thing.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach.** Coverage is collected from each source as Go *binary*
coverage (`GOCOVERDIR` / `go build -cover`) rather than as separate text
profiles, then merged with `go tool covdata merge`, which **unions** counters —
so "reached by ANY source" is the merge semantic we want, for free. The merged
set is converted with `covdata textfmt` and handed to `scupper`.

`-covermode=set` throughout: reachability is a boolean "did this line run"
question, `set` answers exactly that, and all legs must share one mode for
`covdata merge` to accept them.

**Why graceful shutdown is a prerequisite, not a drive-by refactor.** A
`-cover`-instrumented binary writes its counters only on a *clean* exit. A
rela-server that is killed at the end of an e2e run loses **all** of its
coverage, so the e2e leg would silently contribute nothing. Handling
SIGTERM/SIGINT is what makes that leg real.

**Alternatives rejected:**
- *Enforce a threshold now* — rejected: the honest baseline is unknown until
  measured, so any number would be invented.
- *Make the CI job required* — rejected: a first pass that can block merges on a
  number nobody has reviewed yet is hostile.
- *Single `go test -coverprofile` run* — rejected: attributes only same-package
  unit coverage, so cross-package and e2e execution read as unreached.

**Files to modify:** `scripts/reachability.sh` (new), `justfile`,
`.github/workflows/ci.yml`, `.gitignore`, `cmd/rela-server/main.go` (shutdown),
`e2e/tests/fixtures.ts` (coverdir), plus dismissal comments across `cmd/` and
`internal/`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The script's only input is an optional
`--threshold N`; unknown arguments are rejected with exit 2. Coverage data is
produced by the local toolchain, not by untrusted input.

**Security-Sensitive Operations:** `scupper` is pinned to an exact tag
(`v0.2.0`) rather than `@latest`, so CI does not silently execute a new upstream
version. Temporary coverage directories are created with `mktemp -d` and removed
by an `EXIT` trap. The graceful-shutdown change alters process lifecycle only —
no auth, crypto, or request-handling path is touched.

**Note.** Dismissals can only *remove* blocks from the count; a
never-executed, non-dismissed statement still counts against the number, so a
directive cannot mask a genuine gap.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. AC1 — run the script end-to-end on a clean tree; expect a reachability
   percentage and a merged-sources line.
2. AC2 — run with no `--threshold`; expect exit 0 regardless of the number.
3. AC3 — `--require-reason` is passed unconditionally; a bare directive is a
   hard error (exit 2) naming the offending line.
4. AC4 — rela-server exits cleanly on signal and its coverage lands.
5. AC5 — `go build ./...`, `go test ./...`.

**Edge Cases:**
- No coverage collected at all → script exits 2 with "no coverage data
  collected" rather than reporting a meaningless 0%.
- Postgres/e2e legs absent (no `RELA_TEST_DATABASE_URL`, `RUN_E2E` unset) → legs
  are skipped with an explicit message, and only the dirs that actually produced
  `covmeta.*` are merged.
- Unbalanced `-start`/`-end` directives → hard error naming the line; scupper
  never silently swallows a mistyped directive.

**Negative Tests:** unknown script argument → exit 2. Below-threshold run (when
a threshold is eventually set) → exit 1.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- *Job runtime.* The `-coverpkg=./...` leg is slower than a plain test run.
  Mitigated by leaving e2e and postgres legs opt-in in CI.
- *Baseline drifts before a threshold lands.* Accepted: report-only output is
  advisory, and the artifact is uploaded on every run so the trend is visible.
- *Supply chain.* Mitigated by the exact version pin.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** N/A for user-facing docs — this adds a CI signal and a
developer recipe, no CLI, API, or operator behavior change. The rationale that
needs to be written down (floor vs. quality, why report-only, why
`coverage-ignore`) lives in the script header, the justfile comment, and the CI
job comment, where the reader encounters it.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None recorded as review-responses. The one design
question that could have sunk the work — which directive spelling the pinned
tool actually honors — was settled empirically against v0.2.0 before
implementation (see Research).
