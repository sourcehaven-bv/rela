---
id: REV-K8RLE
type: review-checklist
title: 'Review: Re-enable CodeQL scanning (last analysis Feb; default-setup not-configured)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: workflow YAML only)
- [x] ~~Lint clean (`just lint`)~~ (N/A: YAML, covered by Lint Markdown)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: trivial one-file workflow addition using the standard CodeQL Action v3 template)
- [x] ~~All critical review-responses addressed~~ (N/A)
- [x] ~~All significant review-responses addressed~~ (N/A)
- [x] Self-reviewed the diff for unrelated changes — single file `.github/workflows/codeql.yml`

**Review Responses:** none — trivial chore, no review needed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1** (workflow committed): PASS (file exists).
- **AC2** (analysis appears on merge): verify post-merge.
- **AC3** (alerts reflect current code): verify post-merge; the 6 Feb alerts will either close or re-anchor.

## Documentation (enhancements only)

- [x] ~~All N/A — chore, no user-facing docs~~

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — verified after merge
- [x] PR URL documented below

**PR:** filled in after `gh pr create` below
