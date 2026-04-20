---
id: IMPL-2RG24
type: implementation-checklist
title: 'Implementation: Replace backend per-file coverage ratchet with package floors; add govulncheck + gosec CI gates'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: CI-config-only refactor, no application code changed)
- [x] ~~Integration tests written~~ (N/A: CI changes verified by running `just ci` locally and by the PR's own CI run)
- [x] Happy path implemented
- [x] Edge cases from planning handled (idempotent issue-on-failure; frontend-only post-merge-sync path)
- [x] Error handling in place (`scripts/govulncheck-filtered.sh` exits non-zero; issue-create has `set -euo pipefail`)

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
- `just coverage-check` → `Package coverage threshold (0%) satisfied: PASS`, `Total coverage threshold (65%) satisfied: PASS`, `Total test coverage: 71.8% (12566/17502)`.
- `just build` → all three binaries built (CLI, server, desktop).
- YAML syntax check: `python3 -c "import yaml; yaml.safe_load(...)"` on all three modified workflows — passed.
- `grep coverage-baseline .github/ justfile .testcoverage.yml CLAUDE.md` — only legitimate frontend/planning-doc references remain.
- `grep -i ratchet CLAUDE.md` — no backend-section matches; one frontend reference in the new section explaining why frontend still uses one.

**Acceptance criteria mapping:**

| AC | Evidence |
|---|---|
| 1. `just coverage-check` passes on develop, fails below floor | `Total coverage threshold (65%) satisfied: PASS` locally |
| 2. `.coverage-baseline` deleted, no backend refs | `ls .coverage-baseline` → not found |
| 3. `baseline-guard` job removed | `grep "baseline-guard:" .github/workflows/ci.yml` → only `frontend-baseline-guard` remains |
| 4. `post-merge-sync.yml` frontend-only | Inspected: only `frontend/.coverage-baseline` referenced |
| 5. `govulncheck` required on PR | New `vulncheck` job in `ci.yml`, added to `build.needs` |
| 6. Weekly failure files issue | New step in `security.yml` with idempotent `gh issue list`/`create`/`comment` |
| 7. Codecov upload removed | `grep -r codecov .github/workflows/` → no matches |
| 8. `just govulncheck` exists | Verified recipe works |
| 9. CLAUDE.md updated | Rewrote "Test Coverage"; added "Security Checks" section |

## Quality

- [x] Code follows project patterns (inline `gh` CLI usage matches `post-merge-sync.yml` style)
- [x] No security issues introduced (strengthens posture: blocking PR vulncheck + weekly CVE issue-filing)
- [x] No silent failures
- [x] No debug code left behind
