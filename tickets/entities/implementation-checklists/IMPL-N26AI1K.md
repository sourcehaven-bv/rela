---
id: IMPL-N26AI1K
type: implementation-checklist
title: 'Implementation: A body line starting with a conflict marker makes the file unreadable'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestConflictBlocks, TestParseDocument_ConflictMarker, TestRemoveConflictMarkers_KeepsDocumentedExample)
- [x] Integration tests written (test full flow, not just units) (storetest RoundTripsBodyDocumentingConflictMarkers on every backend; TestWrite_ConflictBlockBodyStaysReadable)
- [x] Happy path implemented
- [x] Edge cases from planning handled (lone marker, out-of-order markers, CRLF, diff3, merge inside a code block, fence in a frontmatter block scalar, setext underline)
- [x] Error handling in place (errors surfaced, not swallowed) (fsstore refuses a document that would read as conflicted)

## Test Quality

- [x] Using fixture builders or factories for test data (newTestStore, storetest factory)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (store round trip through fsstore, memstore, sqlitestore, pgstore conformance)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** The storetest round trip failed on fsstore before the
fix and passes on every backend now. go test on markdown, conflict and store
passes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (three detectors replaced by
markdown.ConflictBlocks)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
