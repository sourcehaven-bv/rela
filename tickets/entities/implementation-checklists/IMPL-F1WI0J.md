---
id: IMPL-F1WI0J
type: implementation-checklist
title: 'Implementation: Comment and relation counts on kanban cards'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Store: `comments.Store.Count` on mem, file, pg and sqlite, pinned by
`commentstest.RunCountTests` (per face, resolved included, empty input, more
targets than one SQLite chunk) and a case test in `RunKeyFidelityTests` (SQL
backends; the file backend inherits the filesystem's case rules). Handler:
`comment_counts_test.go` (global grant, not requested, unparseable flag, no
`comment:read`, local-role-only, commenting off, type not commentable) and
`TestQueryBudget_CommentCountsAreOneCallPerPage` (one comment-store call and
`listPageBudget` graph reads at 10 and 50 rows). Config:
`TestValidateKanbans_CardFieldCounts` plus calendar and gantt refusals.
Frontend: `KanbanView.cardCounts.test.ts`. E2E: `kanban-card-counts.spec.ts`. A
failed count is a 500 `comments_failed`, not an absent field.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- AC1 relation count: e2e board `feature-counts`; FEAT-001 (implemented by
TASK-001) shows "1" with the git-branch icon; FEAT-002 shows none. Screenshot
`.ignored/kanban-card-counts.png`.
- AC2 comment count: e2e adds two comments to FEAT-002; the card shows "2" with
the comment icon; the request carries `comment_counts=true`.
- AC3 batching: budget test, 1 Count call at 10 and 50 rows; graph reads equal
to a plain list page.
- AC4 ACL: handler test cases for no `comment:read` and local-role-only return
no `_comment_count`; mutation check (gate replaced by `true`) fails both.
Relation counts read the gated `relations` map.
- AC5 config: table test covers display without relation, unknown display,
comments with property/relation, comments on a non-commentable type; calendar
and gantt refuse both keys.
- pgcomments run against a local PostgreSQL 18 (RELA_TEST_DATABASE_URL), all
pass including Count; sqlite and file backends pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns: `wantCommentCounts` mirrors `wantContent`; `serveCommentCounts`
mirrors `serveOwners`; `RowCounts` is embedded like `EditState` to keep
`v1.Entity` under its plimsoll field cap. golangci-lint, arch-lint, plimsoll,
comment-lint, markdownlint, eslint (0 errors) and vue-tsc are clean.
