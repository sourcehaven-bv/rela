---
id: IMPL-2RG24
type: implementation-checklist
title: 'Implementation: Replace backend per-file coverage ratchet with package floors; add govulncheck + gosec CI gates'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: CI-config-only refactor, no application code changed)
- [x] ~~Integration tests written~~ (N/A: CI verified by running `just ci` locally and by the PR's own CI run)
- [x] Happy path implemented
- [x] Edge cases from planning handled (idempotent issue-on-failure; frontend-only post-merge-sync path; path-filter on PR vulncheck; auto-update verification re-runs govulncheck)
- [x] Error handling in place

## Test Quality

- [x] ~~Using fixture builders~~ (N/A: no test data changes)
- [x] ~~No hardcoded values in assertions~~ (N/A)
- [x] ~~Only specifying values that matter~~ (N/A)
- [x] ~~Interpolated values constructed from objects~~ (N/A)
- [x] ~~Property comparisons use original object~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

- `just lint` → `0 issues.`
- `just govulncheck` → `govulncheck: no actionable vulnerabilities found.` Ignored: `GO-2026-4923`.
- `just coverage-check` → `Total coverage threshold (65%) satisfied: PASS`, `Total test coverage: 71.8% (12566/17502)`.
- `just build` → all three binaries built.
- YAML syntax: all workflows pass `yaml.safe_load`.
- `scripts/govulncheck-fixable.sh` unit-tested against synthetic JSON fixtures:
  - fixable + called + not-ignored → emits `module version`, exit 0
  - fixable + called + ignored → no output, exit 1
  - called but no `fixed` event (upstream-unfixed) → no output, exit 1
  - empty input → no output, exit 1
- Live CI on PR #483: first-ticket scope all green (12/12 checks); follow-up scope pushed after.

**Acceptance criteria mapping:**

| AC | Evidence |
|---|---|
| 1. `just coverage-check` passes, fails below floor | `Total coverage threshold (65%) satisfied: PASS` |
| 2. `.coverage-baseline` deleted, no backend refs | `ls .coverage-baseline` → not found |
| 3. `baseline-guard` job removed | only `frontend-baseline-guard` remains |
| 4. `post-merge-sync.yml` frontend-only | inspected |
| 5. `govulncheck` required on dep-touching PR | `vulncheck` job with paths-detect, added to `build.needs` |
| 6. Weekly failure surfaces automatically | **Upgraded:** attempts auto-update + merge via App token; falls back to deduplicated issue if no upstream fix |
| 7. Codecov upload removed | none |
| 8. `just govulncheck` exists | verified |
| 9. CLAUDE.md updated | rewrote Test Coverage; added Security Checks with auto-update description |

**Follow-up scope (added after initial PR review):**

- `scripts/govulncheck-fixable.sh` — parses govulncheck JSON, emits `<module> <fixed-version>` for called-and-fixable vulns, honors `IGNORED_OSVS`
- `.github/workflows/ci.yml` vulncheck — path-filter (only runs on PRs touching `go.mod` / `go.sum`)
- `.github/workflows/security.yml` — App token (`APP_ID` / `APP_PRIVATE_KEY`); on vuln detection: run `go get`+`tidy`, re-verify, push branch, open PR with auto-merge; fall back to issue-filing if not resolvable

## Quality

- [x] Code follows project patterns (App token + `gh pr merge --auto` mirrors `post-merge-sync.yml` and `dependabot-auto-merge.yml`)
- [x] No security issues introduced (strengthens posture; auto-update PR still goes through full CI incl. tests/lint/vulncheck before merge)
- [x] No silent failures
- [x] No debug code left behind
