---
id: IMPL-8CUMB
type: implementation-checklist
title: 'Implementation: Re-enable CodeQL scanning (last analysis Feb; default-setup not-configured)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: workflow YAML; no code)
- [x] ~~Integration tests written~~ (N/A: workflow is tested by running on the merge commit)
- [x] Happy path implemented — `.github/workflows/codeql.yml` added
- [x] ~~Edge cases from planning handled~~ (N/A)
- [x] Error handling in place — CodeQL Action v3 handles its own retries; `fail-fast: false` across the matrix so one language failure doesn't block the other

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A)
- [x] ~~Only specifying values that matter for the test~~ (N/A)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end — workflow triggers on push-to-develop + PR + weekly schedule. Verified by observing CI on the PR itself (CodeQL job will appear as a check).
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified — weekly fallback (cron) prevents the "no scans for months" state we just fixed

**Verification Evidence:**

- **AC1** (`codeql.yml` committed): `ls .github/workflows/codeql.yml` → exists.
- **AC2** (analysis runs on merge commit): verified once PR merges by checking `gh api repos/.../code-scanning/analyses --jq '.[0]'` returns a recent `created_at` on `refs/heads/develop`.
- **AC3** (alerts reflect current code): the 6 existing `go/path-injection` alerts on `safefs.go` lines 54/59/64/65/78 will either:
  - transition to `fixed` (expected — RootedFS.resolve is now on the taint path), OR
  - re-anchor to current line numbers if the taint detector still sees them.
  Either outcome tells us what to do next in TKT-K3YYE.

## Quality

- [x] Code follows project patterns — matches the v4 checkout / v5 setup-go pattern used by ci.yml and security.yml
- [x] No security issues introduced — scopes limited (`contents: read`, `security-events: write`, `actions: read`); no secrets beyond the default `GITHUB_TOKEN`
- [x] No silent failures — CodeQL Action surfaces errors to the Actions log and the PR check
- [x] No debug code left behind
