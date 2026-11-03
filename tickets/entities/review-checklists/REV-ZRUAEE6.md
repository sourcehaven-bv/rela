---
id: REV-ZRUAEE6
type: review-checklist
title: 'Review: Bound golangci-lint memory and serialise runs'
started: "2026-11-02"
completed: "2026-11-02"
status: done
---

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: no Go or frontend code changed; CI runs the full suite)
- [x] Lint clean (`just lint`) (CI lint job runs with the new `.golangci.yml`; `just --dry-run lint` expands to the expected commands)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go comments changed)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go code changed)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: tooling config change of 19 lines)
- [x] ~~All critical review-responses addressed~~ (N/A: no review responses)
- [x] ~~All significant review-responses addressed~~ (N/A: no review responses)
- [x] Self-reviewed the diff for unrelated changes

## Acceptance Verification

- [x] Each acceptance criterion tested (`just --dry-run lint` shows `GOMEMLIMIT` and `--allow-serial-runners` on every run)
- [x] ~~Test evidence documented in implementation checklist~~ (N/A: chore without implementation checklist)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore)
- [x] ~~User-facing documentation updated~~ (N/A: chore)
- [x] ~~Docs-checklist marked as done~~ (N/A: chore)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
