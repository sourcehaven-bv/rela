---
id: IMPL-UW8NO4
type: implementation-checklist
title: 'Implementation: Comments across markdown block boundaries'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (textanchor `crossblock_test.go`, `quotefind/segments_test.go`; `internal/comments` `TestBody_*`; vitest `commentHighlight.blocks.test.ts`)
- [x] Integration tests written (test full flow, not just units) (handler `TestComments_TextAnchorAcrossBlocks`; e2e `comments across a heading and its body`)
- [x] Happy path implemented
- [x] Edge cases from planning handled (code fences and inline code cut out, task checkboxes excluded, table cells split, tight lists, blockquotes, empty segment list, segments outside range ignored, older server without segments)
- [x] Error handling in place (errors surfaced, not swallowed) (unresolvable anchors still report detached; nothing new can fail)

## Test Quality

- [x] Using fixture builders or factories for test data (`crossBlock`, `spanOf`, `newCrossAnchor`, `setBody` helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded (offsets derived from fixtures with Index/spanOf)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran the e2e `rela-server` build against a copy of the tickets project with
comments enabled, driven by a Playwright script (the Chrome extension was not
connected).

- AC1: selected from the `## Configuration` heading into the paragraph. Two
marks rendered, one in the H2 and one in the P, same comment id. Clicking the
paragraph mark opened the thread "Heading plus body".
- AC2: selected across two tight list items. One mark per LI.
- AC3/AC4: edited the file on disk: heading typo, inserted a paragraph inside
the range, changed a word in the body, reflowed the paragraph. Both comments
still highlighted; the heading comment covers the inserted paragraph and is
marked uncertain.
- AC5: rewrote both list items. The list comment rendered no marks (detached);
nothing else was highlighted instead.
- AC6: `make setup` inline code sits outside the marks; vitest and Go segment
tests cover fences and inline code.
- AC7: existing comment e2e specs (7) and handler tests pass unchanged.

`go test -race ./internal/comments/... ./internal/dataentry/...` pass;
golangci-lint 0 issues; `just arch-lint` OK; textanchor `go test -race` and
golangci-lint clean; vitest 31/31; e2e comments.spec.ts 8/8.

## Quality

- [x] Code follows project patterns (check similar code) (prepared-document pattern mirrors `textanchor.Document`; wire type pointer-for-present like Start/End)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (`ResolveText`/`ApplyReplacement` now
delegate to `Body`)
- [x] No security issues introduced (segments are computed from the redacted body only; highlight markup unchanged and still sanitized by DOMPurify)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
