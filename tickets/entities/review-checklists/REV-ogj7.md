---
id: REV-ogj7
status: done
title: 'Review: Allow configuration of short ID capitalization'
type: review-checklist
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-w9u5, RR-2kgk (both addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- PASS: `id_caps: upper` generates uppercase random suffix (TestGenerateID_ShortWithIDCaps)
- PASS: `id_caps: lower` generates lowercase random suffix (TestGenerateID_ShortWithIDCaps)
- PASS: Default is uppercase (TestGenerateID_ShortWithIDCaps/ticket-default)
- PASS: Prefix case preserved (verified in GenerateShortID)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/244
