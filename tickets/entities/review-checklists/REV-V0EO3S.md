---
id: REV-V0EO3S
type: review-checklist
title: 'Review: Release test gate lacks bubblewrap so releases fail on commits that pass CI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — pre-commit hook ran format, lint and tests: all green
- [x] Lint clean — golangci-lint reported `0 issues.` via the pre-commit hook
- [x] ~~Coverage maintained~~ (N/A: no Go code changed — the diff is one workflow file and one docs file)

## Code Review

- [x] ~~Run `/code-review`~~ (N/A: no application code — a 17-line CI workflow change, reviewed directly against `ci.yml` as the reference implementation)
- [x] No critical review-responses raised
- [x] No significant review-responses raised
- [x] Self-reviewed the diff for unrelated changes — `git diff origin/develop` outside `tickets/` is exactly 2 files, +22/-1: the `release.yml` test job and one docs section. No stray edits

**Review Responses:** none — no findings warranted a review-response entity.

## Acceptance Verification

- [x] Each acceptance criterion tested — the release `test` job now matches `ci.yml`'s runner (`ubuntu-26.04`) and both sandbox steps, confirmed by re-parsing the workflow YAML and by direct diff against `ci.yml`
- [x] Test evidence documented in implementation checklist — see IMPL-099MO1 "Verification Evidence"

**Acceptance Status:**

- AC1 — release test gate runs on the same runner as CI: **PASS** (`runs-on: ubuntu-26.04`).
- AC2 — bubblewrap installed before tests run: **PASS** (install step precedes `Run tests`).
- AC3 — a broken/absent sandbox fails loudly rather than silently: **PASS** (`bwrap --unshare-all --ro-bind / / /bin/true` verify step retained from CI).
- AC4 — no other release job regressed: **PASS** (`security`, `release`, `desktop`, `homebrew` untouched; none run sandboxed tests, so `ubuntu-latest` remains correct for them).
- AC5 — the tag builds and publishes assets: **PENDING** — only verifiable by the next release run, since this gate is reachable only from a tag push.

## Documentation (enhancements only)

Skipped: this is a bug fix, not an enhancement. `docs/releasing.md` was still
updated, since that file enumerates release failure modes and this is a new
named one ("Release-runner drift from CI").

- [x] ~~Docs-checklist created and linked~~ (N/A: bug fix, not an enhancement)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing behaviour change — `docs/releasing.md` is a maintainer runbook and was updated)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

## Final Checks

- [x] Commit message explains the why, not just what — names the CI parity requirement and the v26.7.1 consequence, not just "install bubblewrap"
- [x] No TODOs or FIXMEs left unaddressed — the deferred `workflow_call` refactor is recorded in the bug's `prevention` and the measure's description, not left as an inline TODO
- [x] Ready for another developer to use — both added steps carry comments explaining why they exist and that they must track `ci.yml`

## Pull Request

- [x] PR created against `develop`
- [ ] All CI checks pass — see PR
- [x] PR URL documented below

**PR:** see the `fix/release-test-sandbox` PR linked on BUG-2J30F3.
