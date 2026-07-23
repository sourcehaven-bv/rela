---
id: REV-XMVHEE
type: review-checklist
title: 'Review: Split docs build into a separate rela-docs binary (unlink chromedp from rela)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — 76 packages, 0 FAIL
- [x] Lint clean (`just lint`) — 0 issues
- [x] Coverage maintained (`just coverage-check`) — PASS, total 76.4%
- [x] arch-lint clean · plimsoll PASS · `go vet ./...` clean · all build tags compile

## Code Review

- [x] Run code review (cranky-code-reviewer + go-architect, in parallel on the staged diff)
- [x] All critical review-responses addressed (none critical)
- [x] All significant review-responses addressed (RR-CWEDAQ, RR-V9KEES, RR-0AJ0PI)
- [x] Self-reviewed the diff for unrelated changes (diff is scoped; only the split + shared error extraction)

**Review Responses:** RR-CWEDAQ (significant, addressed), RR-V9KEES
(significant, addressed), RR-0AJ0PI (significant, addressed), RR-1NTHFT (minor,
addressed). go-architect: no blocking findings; endorsed the consumer-side
`Project` interface and build-tag seam.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. chromedp isolation — **PASS**: `rela`=0, `rela-server`=0, `rela-docs`=62, `-tags postgres rela-docs`=0 (CI assertion added).
2. `rela` back to ~47 MB — **PASS**: 46.8 MB (was 62).
3. `rela-docs build` renders identically incl. screenshot{} — **PASS**: live render verified; docscli tests cover typeref/roles_matrix/description/screenshot fail-loud.
4. All build-tag combos compile — **PASS**: default, memorybackend, postgres.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created~~ (N/A: internal refactor, not an enhancement) — but user-facing docs updated anyway:
- [x] GUIDE-rela-docs.md uses `rela-docs build` as the command from the outset (never framed as a migration — the feature is unreleased) + build rationale; docs/rela-docs.md regenerated; example manual reference updated.

## Final Checks

- [x] Commit message explains the why (unlink chromedp, binary-size motivation)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** *to be created via `/pr` — the branch `tkt-x00cdi-rela-docs-binary` is
committed and all local gates are green*
