---
id: IMPL-1VL3CM
type: implementation-checklist
title: 'Implementation: The release workflow''s gate jobs cannot build rela-desktop'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no Go code changed; the change
is two `apt-get install` steps in a GitHub Actions workflow)
- [x] ~~Integration tests written~~ (N/A: the only integration is the release
workflow itself, which cannot be executed without pushing a tag — that gap is
the subject of the measure this bug adds)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place~~ (N/A: declarative workflow steps, no error
paths to surface)

The change is confined to `.github/workflows/release.yml`:

- the Test job's existing bubblewrap install becomes
`bubblewrap libgtk-4-dev libwebkitgtk-6.0-dev`, matching `ci.yml:49-54`
- the Security job gains an `Install Wails Linux dependencies` step before
govulncheck, matching `security.yml:55-58`

Both are line-for-line mirrors of steps that already run successfully on
`ubuntu-latest` and `ubuntu-26.04` respectively, which is the evidence that the
packages resolve on those runners.

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests)
- [x] ~~No hardcoded values in assertions~~ (N/A: no tests)
- [x] ~~Only specifying values that matter~~ (N/A: no tests)
- [x] ~~Interpolated values constructed from objects~~ (N/A: no tests)
- [x] ~~Property comparisons use original object~~ (N/A: no tests)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The defect was confirmed from the failing run before changing anything. Run
34500494895 (tag `v26.9.4`): Test and Security both `failure`, Release
`skipped`. The Test job's failure is `FAIL
github.com/Sourcehaven-BV/rela/cmd/rela-desktop [build failed]` preceded by
`Package 'gtk4' not found`; the Security job's is `could not import C (no
metadata for C)` at govulncheck's package load. No test assertion failed in
either.

The same failure was confirmed on the two earlier tags, so this is one cause and
not three coincidences: run 34270668114 (`v26.9.2`) and run 34450836963
(`v26.9.3`) both show the identical Test+Security failure with Release skipped.
`gh release view` confirms all three tags have no release object.

The fix was checked against the workflows that already work: `ci.yml` installs
`libgtk-4-dev libwebkitgtk-6.0-dev` and its Test job passes on this same commit;
`security.yml` installs the same pair and its govulncheck passes. The edited
YAML parses (`yaml.safe_load`) and the job/runner mapping was re-read after
editing to confirm the steps landed in the intended jobs.

**What is not verified:** that the release workflow now succeeds. It only runs
on a tag push, so the proof is the next tag. This is stated rather than implied
because a green PR here does not exercise the changed file at all.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures
- [x] No debug code left behind

The DRY finding is the substance of this bug and is deliberately **not** acted
on in this PR. The right fix is one `workflow_call` gate shared by ci.yml and
release.yml instead of three copies; doing that here would turn a one-line
release unblock into a CI refactor, and the release is currently broken. The
finding is recorded as `release-gates-match-ci-build-prerequisites` (status
`proposed`) so the green build does not imply the class is closed.

No security impact: the change installs two distribution packages from the
runner's configured apt sources and interpolates no untrusted input. The
`govulncheck` scan coverage is *widened* by the fix, since the desktop packages
now load instead of erroring out.
