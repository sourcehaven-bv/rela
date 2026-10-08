---
id: REV-Y7THZ7
type: review-checklist
title: 'Review: Comments across markdown block boundaries'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (full run green; after review fixes, race tests for internal/comments and internal/dataentry, textanchor `go test -race`, vitest 50 highlight tests and the full frontend suite, e2e comments.spec.ts 8/8)
- [x] Lint clean (`just lint`) (0 issues; frontend lint 0 errors, its warnings are in untouched files; `just arch-lint` OK; textanchor golangci-lint 0 issues)
- [x] Comment lint gate clean (`just comment-lint`) (no unresolvable doc links; no advisory findings in files this diff touches)
- [x] Coverage maintained (`just coverage-check`) (total 82.7%, all floors pass)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer plus rela-security-reviewer)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes (reverted Prettier churn in EntityDetail.vue and the e2e files)

**Review Responses:** RR-3CU4SL RR-3NT53X RR-6EWIUT RR-99HT5Z RR-9IISCW
RR-9W4FW2 RR-ACIIJV RR-C9DLVF RR-DMKU4D RR-EKUKEW RR-FWO810 RR-G0VIQB RR-G16D9C
RR-GXLMSR RR-H4PNUY RR-HMWCKQ RR-JEQKJP RR-OCGQNL RR-P2TWCA RR-RUD546 RR-T651VP
RR-TE69HT RR-TNCBS7 RR-TNH89J RR-U9QM0V RR-V2T0KB RR-ZH7QSP

Design review (planning): RR-6EWIUT … RR-U9QM0V. Security review: RR-EKUKEW
(addressed; follow-up TKT-D4EEYS), RR-V2T0KB (deferred until textanchor v0.3.0
is tagged). Code review: RR-H4PNUY (critical), RR-99HT5Z, RR-OCGQNL, RR-ZH7QSP,
RR-HMWCKQ, RR-TNCBS7, RR-3CU4SL (significant), all addressed; four minor
addressed; one nit addressed, one wont-fix.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 PASS: e2e "comments across a heading and its body"; manual run showed H2 and P marks with one id, and clicking the P mark opened the thread.
- AC2 PASS: two tight list items marked per LI (manual); paragraphs and lists in vitest and the golden test.
- AC3 PASS: reflow, inserted paragraph, moved section (TestResolveCrossBlockSurvivesEdits; manual edit on disk).
- AC4 PASS: heading typo, body word change, long-endpoint edit (textanchor tests; handler test "still resolves after an edit inside the range"; e2e edit step).
- AC5 PASS: TestResolveCrossBlockOrphans and TestResolveCrossBlockNeverMisplaces (rewritten or deleted middle, renamed short heading, rewritten long endpoint); manual rewrite of list items detached the comment.
- AC6 PASS: fenced and inline code excluded (segments tests, golden fixture, handler test "a range over only code has an empty segment list").
- AC7 PASS: existing comment e2e, handler and resolver tests pass unchanged.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (docs/comments.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-7RD2GA

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: /pr runs after the ticket is done; PR and CI status are recorded on GitHub per TKT-UFV01M)
