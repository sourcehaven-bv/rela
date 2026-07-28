---
id: REV-H9QLRQ
type: review-checklist
title: 'Review: Native in-process image processing: decode-verify, EXIF-orientation, re-encode (Phase 1)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full suite green, 73 packages; `-race` clean on touched packages; both new fuzzers clean (1.6M+ execs)
- [x] Lint clean (`just lint`) — `golangci-lint` 0 issues on all touched packages; `just arch-lint` OK; `just plimsoll` OK
- [x] Coverage maintained (`just coverage-check`) — PASS; `internal/imgproc` 88.1% vs 78% floor; total 76.4%

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** All addressed. Design-review (7): [[RR-7R3EYR]] [[RR-4G5YBU]]
[[RR-S8FEUJ]] [[RR-ZM3PE7]] [[RR-01E7DG]] [[RR-U517I0]] [[RR-9IVLFO]].
Code-review (5): critical [[RR-YLYWUZ]] (GIF memory bomb → allocation-free frame
counter); significant [[RR-K5757G]] (EXIF parser recover guard), [[RR-TEJ60P]]
(direct EXIF fuzz + GIF tests); minor [[RR-DR2DR7]] (concurrency ceiling),
[[RR-2ITK58]] (ErrTimeout→5xx, quality validation). No open critical/significant.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** All 13 ACs PASS — evidence per-AC in [[IMPL-2SBIJU]].
Highlights: AC2 orientation always-on (`TestNormalize_AppliesOrientation_EndToEnd`),
AC4 bomb guard (`TestNormalize_PixelCap`), AC5 crafted-input safety (fuzz + race),
AC13 animated-GIF rejected cheaply (`TestGIFBomb_RejectedWithoutDecode`, ~350µs).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** [[DOCS-V8M85I]]

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- filled after creation -->
