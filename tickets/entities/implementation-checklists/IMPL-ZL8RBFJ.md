---
id: IMPL-ZL8RBFJ
type: implementation-checklist
title: 'Implementation: Run E2E and Frontend CI jobs in the Playwright image'
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: CI workflow change, no Go or TS code)
- [x] ~~Integration tests written~~ (N/A: the CI runs on the PR exercise the change)
- [x] Happy path implemented
- [x] Edge cases from planning handled (retry loops exit non-zero after the third failure)
- [x] Error handling in place (each install step fails loudly)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests changed)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests changed)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests changed)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests changed)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests changed)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** actionlint reports no new findings. The image tag
matches the frontend lockfile (playwright 1.64.0) and the digest matches the
registry. CI on the PR runs Frontend in the container and E2E with retries.

## Quality

- [x] Code follows project patterns (the Semgrep job already uses `container:`)
- [x] ~~Checked for DRY opportunities~~ (N/A: the apt retry loop is per step; a composite action would be a refactor out of scope)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
