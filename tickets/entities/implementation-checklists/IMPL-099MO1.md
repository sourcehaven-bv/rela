---
id: IMPL-099MO1
type: implementation-checklist
title: 'Implementation: Release test gate lacks bubblewrap so releases fail on commits that pass CI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: CI workflow configuration change, no Go code — the control is the workflow definition itself, see `release-test-gate-sandbox-parity`)
- [x] ~~Integration tests written~~ (N/A: the release gate only executes on a tag push and cannot be exercised from a PR — this is why4/why5 of the bug, recorded rather than papered over)
- [x] Happy path implemented — `release.yml` test job pins `ubuntu-26.04` and installs bubblewrap, mirroring `ci.yml`
- [x] Edge cases handled — kept CI's `bwrap --unshare-all --ro-bind / / /bin/true` verification step so a runner where the sandbox is present but non-functional (the AppArmor `apparmor_restrict_unprivileged_userns` case the `ubuntu-26.04` pin exists for) fails loudly rather than mass-skipping
- [x] Error handling in place — the verify step makes an absent/broken sandbox a hard named failure instead of 35 confusing `exec: "bwrap": executable file not found` errors that read like a code defect

## Test Quality

- [x] ~~Using fixture builders~~ (N/A: no test code added)
- [x] ~~No hardcoded values in assertions~~ (N/A: no test code added)
- [x] ~~Only specifying values that matter~~ (N/A: no test code added)
- [x] ~~Interpolated values constructed from objects~~ (N/A: no test code added)
- [x] ~~Property comparisons use original object~~ (N/A: no test code added)

## Manual Verification

- [x] Feature manually tested end-to-end — root cause confirmed from the failed run's logs before changing anything; `release.yml` re-parsed with a YAML loader after the edit to confirm the job structure (`runs-on: ubuntu-26.04` plus the 5 steps in order)
- [x] Each acceptance criterion verified — the release `test` job now matches `ci.yml`'s runner and sandbox setup step-for-step
- [x] Edge cases verified — audited the other release jobs (`security`, `release`, `desktop`, `homebrew`); none run sandboxed tests, so `ubuntu-latest` stays correct for them and the change is scoped to the one job that needed it

**Verification Evidence:** Failed run 30433648340 on tag `v26.7.1`: `Test`
failed, so `Release` / `Desktop` / `Update Homebrew Cask` were all skipped and
no release object was created. 35 tests failed across `internal/attachment`,
`internal/cli`, `internal/cmdexec`, `internal/dataentry` and
`internal/transform`; every one traces to
`cmdexec: start "bwrap": exec: "bwrap": executable file not found in $PATH`
(the `internal/dataentry` export failures are 500s downstream of the same
sandbox, since export shells out through `cmdexec`). The identical commit
`ba0c1fd7` passed `CI` on `develop` minutes earlier — the signature of runner
drift, not a code defect. Diffing the two workflows confirmed `ci.yml` installs
and verifies bubblewrap on `ubuntu-26.04` while `release.yml` did neither on
`ubuntu-latest`. Full verification is the next release run, since the gate is
only reachable from a tag push.

## Quality

- [x] Code follows project patterns — copied `ci.yml`'s runner pin, install step and verify step verbatim, including the comment explaining why `ubuntu-26.04` rather than `ubuntu-latest`
- [x] Checked for DRY opportunities — the real DRY fix is a `workflow_call` gate shared by both workflows, which would make this class of drift structurally impossible. Deliberately **not** done here: it is a refactor of both workflows and the immediate need was to unblock the release. Recorded in the bug's `prevention` and in the measure's description rather than silently dropped
- [x] No security issues introduced — the added steps are static `apt-get` / `bwrap` invocations with no `github.event.*` interpolation, so no workflow-injection surface. Installing bubblewrap *strengthens* the gate: it makes the negative sandbox tests (asserting egress and out-of-allowlist reads are blocked) actually run instead of failing closed
- [x] No silent failures — the verify step is the explicit guard against the silent case
- [x] No debug code left behind
