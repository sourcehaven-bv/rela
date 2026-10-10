---
id: PLAN-YUZ9UZ
type: planning-checklist
title: 'Planning: Comment and relation counts on kanban cards'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

A kanban card shows property values and relation targets, never a count. The
Atlas board mockup (TASK-T7342) shows a comment count and a subtask count on
each card. The library card already renders both (`RlMetaItem` with
`message-square` / `git-branch` icons in `RlTaskCard`).

**Scope:**

In: two new card field kinds, config validation, a batched comment count on the
list endpoint, docs, e2e. Out: counts on list/table views and the swimlane
chips; live count updates other than the existing SSE refetch; counting resolved
and open comments separately (the count is all comments on the entity's face).

**Acceptance Criteria:**

1. `- relation: subtask-of` with `direction: incoming` and `display: count`
shows the number of related entities the reader can see. Test: e2e board card
shows "2" with the subtask icon for a fixture with two subtasks.
2. `- comments: true` shows the number of comments on the card's entity.
Test: e2e adds a comment, the card shows "1".
3. Comment counts come from one comment-store call per page. Test: budget
test with a counting comment store, 10 and 50 rows, same call count.
4. A hidden related entity is not counted; a reader without global
`comment:read` gets no count. Tests: Go handler test with ACL; frontend relation
count reads the already gated `relations` map.
5. `display` on a property field, `display` other than `count`, `comments`
combined with `property`/`relation`: config error at load. Test: table-driven
validate test.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (approach follows two existing list-endpoint patterns)

**Existing Solutions:**

- Opt-in per-page data: `include_content` (`wantContent`, `loadRowContent`,
`internal/dataentry/rowcontent.go`).
- Batched per-page fill: `serveOwners` / `setOwners` (`owner.go:202`).
- The list response already carries outgoing AND incoming edges per row,
gated by `visibleRelationIDs`, so a relation count needs no server work.
- `comments.Store` has no count; `List` per target would be one call per row.
- Library card: `RlTaskCard` `commentCount` / `subtaskCount`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Config (`KanbanCardField`): add `Display string` (only `count`, only with
`relation`) and `Comments bool` (alone, optionally with `label`).
`validateKanbans` enforces the combinations.

Relation count: client side. KanbanView already requests rows with their
`relations` map; the count is the length of the gated id list for that relation
and direction. Rendered as `RlMetaItem` with the `git-branch` icon.

Comment count:
- `comments.Store` gains `Count(ctx, keys []string) (map[string]int, error)`,
one query per call: `GROUP BY target_key` in pg and sqlite, a map lookup in mem,
a directory read per key in the file store (the file store is the single-user
tier; no index exists to batch against). Contract test in `commentstest.RunAll`.
- `comments.Service.CountMany` wraps it.
- List endpoint: `?comment_counts=1` (pattern of `include_content`). When
comments are enabled and the principal holds `comment:read` GLOBALLY,
`serveCommentCounts` fills `_comment_count` on every row in one call. Rows
already passed the read gate, so the read floor holds.
- KanbanView sends the param only when a card has a `comments` field.

Alternatives rejected:
- Per-row `Authorizer.CanRead`: it walks ancestors and role relations per
entity (`acl.Request.computeForEntity`), which is a per-row store read the
collection-reads rule forbids. Consequence of the global check: a reader whose
`comment:read` comes only from a local role sees no count on the card (still
sees the thread on the entity page). Fails closed; documented.
- A separate `/_comments/counts` endpoint: a second round trip, and it would
need its own read gate on the ids; the list endpoint already has one.

**Files to modify:**

- `internal/dataentryconfig/config.go`, `validate.go` (+ tests)
- `internal/comments/comments.go`, `service.go`, `commentstest/commentstest.go`
- `internal/comments/{memcomments,filecomments,pgcomments,sqlitecomments}`
- `internal/apiwire/v1/responses.go` (`CommentCount *int` as `_comment_count`)
- `internal/dataentry/api_v1.go`, new `comment_counts.go` (+ tests, budget test)
- `frontend/src/types/config.ts`, `frontend/src/types/entity.ts` (wire type)
- `frontend/src/views/KanbanView.vue` (+ card rendering, unit test)
- `e2e/tests/fixtures.ts`, `e2e/pages/kanban.page.ts`, new spec
- `docs-project/entities/guides/GUIDE-data-entry.md`, API reference

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

`comment_counts` query param: only `1`/`true` turn it on, anything else is off.
Card config: validated at load, unknown `display` values rejected.

**Security-Sensitive Operations:**

Comment count is derived content about an entity. It is computed only for rows
that passed the row gate, and only when the principal holds `comment:read`
globally; otherwise the field is absent. Relation count uses the already
filtered relation ids, so hidden neighbours are never counted. Counts are per
request, never cached across principals.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

See acceptance criteria. Plus the store contract test on all four backends (pg
and sqlite under their build tags in CI).

**Edge Cases:**

- Zero comments or relations: no meta item (the card hides 0).
- Comments disabled on the server: param ignored, field absent.
- Faced entity: counted under its own face key.
- Empty page: no store call.
- Rename: counts follow the existing comment `Rename`.

**Negative Tests:**

- Reader without `comment:read`: `_comment_count` absent.
- Hidden subtask: not counted.
- Invalid config combinations rejected.

## Risk Assessment

- [x] Risks identified with mitigations
- [x] Effort estimated

**Risks:**

1. File-store count is one directory read per row. Mitigation: it is the
single-user tier and only runs when a card asks for it; noted in godoc.
2. Readers with only local-role `comment:read` see no count. Mitigation:
documented; a batched per-entity permission check is a separate ticket.

**Effort:** m
