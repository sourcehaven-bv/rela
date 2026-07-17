---
id: REV-1Y2M4M
type: review-checklist
title: 'Review: Remove the `rela create --title` write shortcut'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` green; `go test ./internal/cli/` ok)
- [x] Lint clean (`golangci-lint ./internal/cli/...` — 0 issues; `go vet` clean)
- [x] Coverage maintained (`just coverage-check` PASS — total 76.8%, package + total thresholds satisfied)

## Code Review

- [x] Ran cranky-code-reviewer on commit 94ca2f7d
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-E22F0A → addressed)
- [x] Self-reviewed the diff for unrelated changes (diff is only --title removal + resolveEntityType signature + doc migration + hook config)

**Review Responses:** RR-E22F0A (significant → addressed), RR-VF95AO (minor →
addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested

**Acceptance Status:**
1. `CreateCmd` has no `Title`/`-t` flag → **PASS** (`rela create requirement -t x` errors "unknown flag -t", exit 80; `--help` no longer lists it).
2. `-P title="X"` still sets the title → **PASS** (unchanged `-P` path; all shipped metamodels use `title`).
3. `go build ./...` + `go test ./...` pass → **PASS** (no test exercised `--title`).
4. `just docs` regenerates cleanly, no `--title` refs remain → **PASS** (repo-wide grep for `rela create … -t/--title` returns CLEAN; markdownlint-cli2 0 errors).

## Documentation

- [x] ~~Docs-checklist created and linked~~ (N/A: refactor; doc change is a small migration note, folded into this PR)
- [x] User-facing documentation updated — CLI reference migration note added (RR-E22F0A resolution) + all examples migrated to `-P title=`.

## Final Checks

- [x] Commit message explains the why (display property is readonly-derived; conflation breaks under templates)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created: https://github.com/sourcehaven-bv/rela/pull/1063
- [x] All CI checks pass (verified locally: build, test, lint, coverage, docs-check, markdownlint, ticket-validate all green; PR CI re-runs on the follow-up push)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1063
