---
id: IMPL-BI76XJ
type: implementation-checklist
title: 'Implementation: CalVer releases (vYY.M.BUILD) with an automated tag cutter'
status: done
---

## Development

- [x] ~~Unit tests written for new code~~ (N/A: shell script + workflow YAML; the
repo has no bats/shell test harness — verified by executing the script against
throwaway git repos, evidence below)
- [x] ~~Integration tests written~~ (N/A: the full flow is a GitHub Actions
workflow; it cannot run until merged to the default branch)
- [x] Happy path implemented (compute next tag → guard → push → summary)
- [x] Edge cases handled (month rollover, numeric sort, alpha/stable collision,
legacy tags, missing tags)
- [x] Error handling in place (unknown flag rejected; `set -euo pipefail`;
tag-exists guard aborts non-zero)

## Test Quality

- [x] ~~Fixture builders or factories~~ (N/A: no test suite — verification is
script execution against throwaway repos)
- [x] ~~No hardcoded values in assertions~~ (N/A: no test suite)
- [x] ~~Only specifying values that matter~~ (N/A: no test suite)
- [x] ~~Interpolated values constructed from objects~~ (N/A: no test suite)
- [x] ~~Property comparisons use original object~~ (N/A: no test suite)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

Script executed against throwaway `git init` repos:

- fresh month, no tags → `v26.7.0`
- after `v26.7.0` → `v26.7.1`; after `v26.7.1` → `v26.7.2`
- `--alpha` → `v26.7.2-alpha`; a following stable run → `v26.7.3` (alpha and
stable share one counter, so they cannot collide on a tag string)
- numeric sort: after `v26.7.9` → `v26.7.10`; after `v26.7.10` → `v26.7.11`
(not string-sorted)
- month rollover: date stubbed to August with July tags present → `v26.8.0`
- scoping: `v26.1.7`, `v25.12.3`, `v0.15`, `v0.9` all correctly ignored

Format constraints verified against the real tooling, not from memory:

- `Masterminds/semver` (the library GoReleaser uses): `v26.7.0` parses with
major=26 / minor=7 / patch=0 and `-alpha` extracted into the prerelease field
- ordering: `v0.15` < `v26.7.0` < `v26.7.12` < `v26.12.0` < `v27.1.0`, and
`git tag --sort=version:refname` agrees
- MSI: `26.7.x` is inside the 255/255/65535 `ProductVersion` maxima, so the tag
is used verbatim in the installer
- nfpm: parses the tag as semver and round-trips it unchanged

## Quality

- [x] Code follows project patterns (app-token step mirrors `security.yml`;
conventional-commit message; workflow comments explain the why)
- [x] Checked for DRY opportunities — choosing `vYY.M.BUILD` removed the entire
MSI-remapping layer an earlier draft needed (a `--msi-version` function, a
fallback branch in `release.yml`, and a docs section)
- [x] No security issues introduced — all workflow inputs are passed via `env:`
rather than interpolated into `run:`, per the actions-injection guidance
- [x] No silent failures (`set -euo pipefail`; the tag-exists guard exits 1;
the only tolerated failure is `git fetch --tags`, which is documented and cannot
lower the counter)
- [x] No debug code left behind
