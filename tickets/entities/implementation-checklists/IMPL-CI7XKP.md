---
id: IMPL-CI7XKP
type: implementation-checklist
title: 'Implementation: Stacked PRs ran zero CI checks, and an empty check list read as green'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: CI workflow trigger change, no Go code — the control is the workflow definition, recorded as `AM-ci-runs-on-every-pr`)
- [x] ~~Integration tests written~~ (N/A: a trigger only fires on a real PR event; it cannot be exercised from within a PR's own jobs. This is why3/why5 of the bug — the failure mode is precisely that no job runs — so it is recorded rather than papered over)
- [x] Happy path implemented — `branches:` filter removed from `on.pull_request` in both `ci.yml` and `codeql.yml`
- [x] Edge cases handled — push triggers deliberately KEEP their `branches:` filters, so the change does not start building every pushed branch; only the PR path, which by definition means "proposed for merge", becomes unconditional
- [x] Error handling in place — n/a for a trigger, but the YAML was re-parsed after the edit to confirm `pull_request` resolves to the unfiltered form (`None`) rather than an empty list, which would have matched nothing

## Test Quality

- [x] ~~Using fixture builders~~ (N/A: no test code added)
- [x] ~~No hardcoded values in assertions~~ (N/A: no test code added)
- [x] ~~Only specifying values that matter~~ (N/A: no test code added)
- [x] ~~Interpolated values constructed from objects~~ (N/A: no test code added)
- [x] ~~Property comparisons use original object~~ (N/A: no test code added)

## Manual Verification

- [x] Feature manually tested end-to-end — confirmed the root cause from `gh run list` before changing anything (only `Dependabot Auto-Merge` had ever triggered on the branch, 3 runs, all `skipped`), then re-targeted the PR to `develop` and confirmed the full check set appears
- [x] Each acceptance criterion verified — audited all 7 workflow files for the same trigger shape; `ci.yml` and `codeql.yml` were the only two filtering `pull_request` on target branch
- [x] Edge cases verified — checked that `security.yml` (the weekly SCA sweep) has no `pull_request` trigger at all and is therefore unaffected; CodeQL's weekly `schedule:` fallback already existed and stays as the backstop

## Quality Checks

- [x] Linter passes — YAML validated with a parser, not by eye
- [x] ~~Type checker passes~~ (N/A: no typed code)
- [x] ~~Coverage thresholds met~~ (N/A: no Go code added)
- [x] No debug artifacts left behind
