---
id: REV-QQRJXN
type: review-checklist
title: 'Review: Postgres derived-schema reconciler (seam + unique rule): atomic unique:true, db status drift, dry-run'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — full `go test -race -shuffle=on ./...` green (0 failures) after `just build` rebuilt the frontend artifacts; the earlier `TestBuiltCSSIsLayered` failure was stale-artifact-only.
- [x] Lint clean (`just arch-lint`, `just plimsoll`, full default-tag `golangci-lint run ./...` = 0 issues; postgres-tag lint clean for all touched files; `lint-md` 0 issues)
- [x] Coverage maintained (package 50% + total 65% thresholds PASS; new branches covered)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer, adversarial pass on the implementation)
- [x] All critical review-responses addressed (RR-5LZWX8/RR-GVXUIQ from design review)
- [x] All significant review-responses addressed (RR-8OIKGN, RR-V08S5M, RR-WF0ZYF from code review)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Design review: RR-5LZWX8, RR-GVXUIQ (critical), RR-AROZJY,
RR-CWI8HG, RR-QY5S4C, RR-2HMGZJ, RR-FTQE3U, RR-B5Y6DZ, RR-3NB0P9 (significant).
Code review: RR-8OIKGN, RR-V08S5M, RR-WF0ZYF (significant), RR-78T6Q9,
RR-0USU3N, RR-DLML7F (minor). All addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (10/10 PASS — see implementation checklist evidence)
- [x] Test evidence documented in implementation checklist

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-EY92SD)
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-EY92SD

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (monitoring)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1371
