---
id: REV-A3SUPNO
type: review-checklist
title: 'Review: Live view misses a change reverted before the next round'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race ./internal/dataentry/`, the only package changed; the full `just test` runs in CI)
- [x] Lint clean (`golangci-lint run --allow-serial-runners ./internal/dataentry/...`: 0 issues; `just arch-lint` clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the change adds tested code to a package above its floor; CI runs the coverage gate)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** design review RR-I4B3MIA, RR-W6ZLXZ7, RR-ZXUOGC2,
RR-ETY1PZZ, RR-UBRB1L3, RR-VBIPRJA, RR-9D340DA, RR-3DQU1D2; code review
RR-FZ013UO, RR-BY5NQTX, RR-IISNHQZ, RR-JD03PNA, RR-U2A6AEJ, RR-GL8RPR0,
RR-JIAF375 and the rest of that batch.

## Acceptance Verification

- [x] Each acceptance criterion tested (a change and its revert between rounds, seen by the client, frames: lease and write tests)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS:
TestLive_LeaseClaimedAndReleasedBetweenRoundsFrames,
TestLive_WriteRevertedBetweenRoundsFrames. PASS: no frame for hidden writes or
other principals' fetches. PASS: presence.spec.ts 60/60.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
