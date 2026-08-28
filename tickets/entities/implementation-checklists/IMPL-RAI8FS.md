---
id: IMPL-RAI8FS
type: implementation-checklist
title: 'Implementation: Reachability floor: merged-coverage pipeline + scupper (report-only baseline)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (see note)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**Note on tests.** The deliverable is a shell pipeline plus CI wiring, not Go
code with its own units — the filtering logic it depends on is tested upstream in
`scupper` (which carries unit, integration-against-the-real-toolchain, and fuzz
tests for its two hand-written parsers). What is verifiable *here* is that the
pipeline runs end-to-end and produces a correct number, which is recorded under
Manual Verification. The Go changes that ship alongside it (rela-server graceful
shutdown) are covered by the existing suite.

**Error handling.** The script is `set -euo pipefail`. It exits 2 on an unknown
argument and 2 when no coverage data was collected at all, rather than reporting
a meaningless 0%. Individual test legs are allowed to fail without aborting the
run (coverage is still collected from the tests that did run) — deliberate, and
called out in the script so it does not read as a swallowed error.

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no Go test data added)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A)
- [x] ~~Only specifying values that matter for the test~~ (N/A)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- **Directive syntax (the decision this work turned on).** Built the pinned
  `scupper@v0.2.0` from source and ran it over a fixture containing one reached
  function and two unreached ones, each dismissed with a *different* spelling:

  ```
  $ scupper -i cover.out                                   # default -d scupper:ignore
  dismissed: 3 statement(s) excluded via scupper:ignore directives
  reachability: 25.0% of statements executed (1/4)

  $ scupper -i cover.out -d coverage-ignore --require-reason
  dismissed: 2 statement(s) excluded via coverage-ignore directives
  reachability: 20.0% of statements executed (1/5)
  ```

  Each invocation honored the spelling matching its `-d` and reported the other
  as unreached — confirming both are live and the choice is a convention, not a
  compatibility constraint. Corroborated in source: `DefaultDirectives(base)`
  derives all five directive forms from the `-d` value.

- **AC1 / AC5 — pipeline against current develop** (after merging 163 commits of
  `origin/develop` into the branch):

  ```
  reachability: 77.1% of statements executed (36805/47755)
  dismissed: 333 statement(s) excluded via coverage-ignore directives
  8411 line-block(s) never executed by any test (10950 statements)
  ```

- **AC2 — report-only.** Run without `--threshold`; exit code 0.
- **AC3 — reasons required.** `--require-reason` is passed unconditionally and
  the run is clean, so all 41 directives (16 `coverage-ignore`, 25
  `coverage-ignore-func`) carry an explanation.
- **AC5 — build and tests.** `go build ./...` clean. `go test ./...` over the
  filtered package list: **exit 0, 95 packages ok, 0 FAIL**.
- **Edge case — skipped legs.** With `RELA_TEST_DATABASE_URL` unset and `RUN_E2E`
  unset, both legs report as skipped and only directories that actually produced
  `covmeta.*` are passed to `covdata merge`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Notes.** `scupper` is pinned to an exact tag rather than `@latest`, so CI does
not silently execute a new upstream version. The version and the directive flags
are each defined once at the top of the script (`SCUPPER_VERSION`,
`DIRECTIVE_ARGS`) and reused by both the threshold and report-only branches.
Temporary coverage dirs are `mktemp -d` with an `EXIT` trap.
