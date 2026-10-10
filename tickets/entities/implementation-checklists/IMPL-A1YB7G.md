---
id: IMPL-A1YB7G
type: implementation-checklist
title: 'Implementation: Data-migration face move and delete leave comment threads behind'
started: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: CLI-only data migration; covered by the command-level test TestMigrateAdoptFace_MovesCommentThreads over assembled services)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: edge cases (dry-run, thread failure, occupied destination, re-run) are pinned by unit tests)

**Verification Evidence:**
- datamigration/comments_test.go: rename_face, migrate_face (apply, dry-run, thread failure), adopt-face, drop_entities and the GC sweep each leave threads at the right address.
- comments/service_test.go TestFaceMoved: field preservation, idempotence, half-finished move, occupied destination.
- cli TestMigrateAdoptFace_MovesCommentThreads: the command passes the comment service through.
- Mutation check: removing moveThreads/dropThreads fails six datamigration tests; removing the CLI wiring fails the CLI test.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
