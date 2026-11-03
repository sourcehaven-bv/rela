---
id: REV-IIREMB
type: review-checklist
title: 'Review: Live views 2/3: per-subscription SSE frames via poke and gated re-read'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (`just coverage-check` runs the full suite with the race detector: all pass; after the architecture fixes the dataentry, principal and changefeed suites were rerun with the race detector and pass; frontend 4043 tests pass)
- [x] Lint clean (`just lint`) (all four build-tag runs clean; arch-lint and plimsoll clean)
- [x] Comment lint gate clean (`just comment-lint`) (no unresolvable doc links; no advisory findings in live*.go, livepath*.go, watcher.go or changefeed)
- [x] Coverage maintained (`just coverage-check`) (package and total floors pass)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer, rela-security-reviewer, and a go-architect architecture review)
- [x] All critical review-responses addressed (none critical)
- [x] All significant review-responses addressed (RR-G03VSY, RR-0JGGEY, RR-HJ310L, RR-LAX75I, RR-GYTAU7, RR-0H0LOU, RR-0YX25V, RR-UC90C0, RR-7AW45P)
- [x] Self-reviewed the diff for unrelated changes (the untracked RlRealtime* Storybook fixtures belong to the realtime research, not this ticket, and are left out of its commit)

**Review Responses:** RR-G03VSY, RR-0JGGEY, RR-HJ310L, RR-LAX75I, RR-GYTAU7,
RR-0H0LOU, RR-0YX25V, RR-UC90C0, RR-7AW45P (significant, addressed); RR-4D7OF8,
RR-U74D0D, RR-SE8UPX, RR-36N6SA, RR-RE0WCF (minor, addressed); RR-TKMXNU (minor,
deferred with reason); RR-6VNZWR (minor, deferred to TKT-H6LD1Y); RR-BMADTF,
RR-02T2UP (nit, addressed); RR-G1EVIB (nit, deferred to TKT-H6LD1Y); RR-MBLOQI
(nit, wont-fix with reason).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 PASS: sse_resume_test.go asserts the `conn` length on the v1 `ready`;
`TestLive_LegacyStreamHasNoConn` covers the legacy stream.
- AC2 PASS: `TestLive_PutValidation`, `TestParseLivePath`.
- AC3 PASS: `TestLive_WriteFramesChangedSubscriptions`.
- AC4 PASS: `TestLive_HiddenWriteSendsNoFrame`.
- AC5 PASS: `TestLive_LosingAccessFrames`.
- AC6 PASS: `TestLive_ViewFramesOnRelatedChange`.
- AC7 PASS: `TestLive_PutOwnership`.
- AC8 PASS: the PUT takes the baseline before 204
(`TestLiveHub_BaselineBeforeAnswer`); the manager calls `onChange` after the 204
(`useLiveSubscription.test.ts`).
- AC9 PASS: `TestLive_ReplaySharing`. One write costs one replay; a burst of
20 costs at most 2, because rounds now run on a fixed clock and a burst may
straddle a tick (RR-0JGGEY).
- AC10 PASS: `TestLive_ReplayMatchesGet`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (api-reference.md, acl-security.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-BYJKHD

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per the user's standing instruction; the ticket id carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the user has not asked to commit or open a PR; run `/pr` when they do)
