---
id: PLAN-Q62CDC
type: planning-checklist
title: 'Planning: MCP tools for comments'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- Five MCP tools on the remote server (rela-server `-mcp`); stdio `rela mcp`
registers only `list_comments` (changed on 2026-10-11, see Out of scope):
`list_comments`, `add_comment`, `update_comment` (body and/or resolved),
`delete_comment`, `accept_comment_suggestion` (added on approval, 2026-10-11).
- Target addressed as `ID` or `ID@face`, the same address grammar the other
MCP tools take. The entity type is taken from the resolved entity, not from the
caller.
- Anchors: `property` (ref = property name), `section` (ref = section slug)
and `text` (quote, quote_prefix, quote_suffix, optional replacement), the same
three kinds and validation as `POST /api/v1/_comments`.
- Identical gating to the HTTP routes: target read floor with uniform
not-found, `comment:read|add|update-own|update-any|delete-own|delete-any`, write
target per face (`face_required` on a faced type addressed by bare id),
read-only instance refusal, `comments_not_enabled` per type.
- Author is the request principal (remote: the verified JWT user).
- Tools are not registered when the project has no `comments:` block
(disabled is absent, as on HTTP).

Out of scope:

- The resolve-check route (`.../resolve`): a UI affordance; an agent learns the
same from `add_comment`'s error.
- Comment counts on `list_entities` / `show_entity`.
- Authoring comments from stdio `rela mcp`: it runs as the process's system
principal, not as a signed-in user. Decided on 2026-10-11 (Jeroen): stdio
does not register the four write tools at all, so no error message is needed;
`list_comments` works there. Choosing a stdio author identity is a separate
decision.

**Acceptance Criteria:**

1. `add_comment` on `PROC-1@concept` with a property anchor creates a
comment authored by the calling principal; `list_comments` on the same address
returns it with anchor, author, created_at, resolved=false.
2. `add_comment` with a text anchor whose quote occurs in the body stores a
text anchor; a quote that does not occur fails with the same message as HTTP
(`invalid_comment`).
3. A principal without `comment:add` gets a forbidden error naming the
permission; a principal that cannot read the target gets the same not-found as
for a non-existent id.
4. `update_comment` with only `resolved=true` resolves without changing the
body; editing someone else's comment without `comment:update-any` is forbidden.
`delete_comment` likewise with `comment:delete-*`.
5. A bare id on a faced type for a write returns `face_required` listing the
readable faces.
6. With no `comments:` block the five tools are absent from `tools/list`;
with comments disabled for one type the tools return `comments_not_enabled` for
that type.
7. On a read-only instance the write tools are refused.
8. HTTP comment behaviour is unchanged (existing dataentry comment tests
pass unmodified).
9. `accept_comment_suggestion` applies an open suggestion to the body and
resolves the comment, with the checks of `POST .../accept`: `comment:read`
plus the target read floor, the entity `update` write (an ACL denial answers
forbidden and is audited), `no_suggestion`, `comment_resolved`,
`suggestion_stale`, and the write target per face.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A. The feature exposes an existing service on an existing
transport; prior art is in-repo.

**Existing Solutions:**

- `internal/dataentry/comments_handler.go` holds all gating and anchor
building for the HTTP routes (`gateCommentTarget`, `gateCommentWriteTarget`,
`gateCommentMutation`, `refuseIfReadOnly`, `buildTextAnchor` in
`comments_wiring.go:251`, `liveAnchors`).
- `internal/comments` already has the transport-neutral pieces:
`Service` (List/Add/Get/Update/Delete), `Authorizer` (`authz.go`,
CanRead/CanAdd/CanUpdate/CanDelete), `ResolveText`.
- `internal/mcp/tools_attachment.go` is the closest MCP analog: side data
on an entity, ACL-gated, with an `AttachmentDeps` bundle injected by both
wirings (`internal/cli/mcp_wiring.go:268`, `cmd/rela-server/mcp.go:73`) and fed
from the data-entry App via `MCPHost` on the remote server.
- Concepts: `mcp-api`; feature FEAT-002WMX (entity commenting).
- No external library applies.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Extract a transport-neutral operations type from the HTTP handler, so
HTTP and MCP share one implementation of the gates: `comments.Ops` in a new file
`internal/comments/ops.go` (or a sibling package `internal/comments/commentops`
if `comments` may not import `visibility`/`entity` resolution; decided during
implementation against `.go-arch-lint.yml`). It takes, as call-site interfaces:
a target resolver (read: address to visible entity; write: write target per
face, the two functions `visibleReader.addressRef` and `writeTargetOr404`
already implement), the comment policy (`metamodel.CommentPolicy`), an
`*Authorizer`, the `*Service`, and a writes-permitted flag. Methods: `List(ctx,
typ, ref)`, `Add(ctx, typ, ref, AddInput)`, `Update(ctx, typ, ref, id, body*,
resolved*)`, `Delete(ctx, typ, ref, id)`. Results carry the resolved anchors
(live/detached) and the editable/deletable flags the HTTP list returns. Errors
are typed sentinels: `ErrTargetNotFound`, `ErrForbidden{Permission}`,
`ErrFaceRequired{Faces}`, `ErrNotEnabled`, `ErrReadOnly`, plus the existing
`ErrNotFound` and validation errors.
Revised during implementation: the operations stay methods of the data-entry
`commentsHandler`, because they need its raw reader, patcher, ACL, audit and
provision seam, none of which `internal/comments` may import. They return a
failure value that the HTTP handler writes with today's writers. The
transport-neutral types (`View`, `AddInput`, `Accepted`, `OpError`) live in
`internal/comments/ops.go`; MCP reaches the operations through
`MCPHost.Comments` and renders `OpError`.
2. Refactor `commentsHandler` to call `Ops` and map the typed errors to the
status codes and titles it returns today. The existing HTTP tests are the guard
that behaviour does not change (AC 8).
3. MCP: new `internal/mcp/tools_comments.go` with the five tools, and a
call-site interfaces `CommentReader` and `CommentWriter` in `Deps` (nil: those
tools not registered). The MCP tool takes `id` (address); the type comes from
the stored row so a caller cannot name a mismatching type. Errors map to `errorResult` texts with the same
wording as the HTTP titles.
4. Wiring: remote via a new `MCPHost.Comments` built from the App's
`commentsHandler` dependencies (live schema, ACL, read-only flag); stdio via
`svc.Comments()` and the stdio reader, list only (`internal/cli/mcp_comments.go`).
5. Add `comments` to `mcp.mayDependOn` in `.go-arch-lint.yml`.
6. Update `internal/mcp/testdata/tools_list.golden.json` and
`docs/mcp-server.md`; add a short MCP section to `docs/comments.md`.

**Alternatives considered:**

- MCP tools calling the HTTP API in-process: rejected, an extra hop and a
second auth path.
- Duplicating the gates in `internal/mcp`: rejected, two copies of
security-relevant code drift (the existing BUG-R1PQY9 face fix shows how easily
one copy is missed).
- Only the remote server: rejected for list; the stdio read path costs
little and keeps the two tool lists aligned.

**Files to modify:**

- new `internal/comments/ops.go` (+ `ops_test.go`)
- `internal/dataentry/comments_handler.go`, `comments_wiring.go`
- `internal/dataentry/mcp_http.go` (MCPHost.Comments)
- new `internal/mcp/tools_comments.go` (+ `tools_comments_test.go`)
- `internal/mcp/server.go` (Deps.Comments), `tools.go` (registration)
- `internal/cli/mcp_wiring.go`, `cmd/rela-server/mcp.go`
- `.go-arch-lint.yml`
- `internal/mcp/testdata/tools_list.golden.json`
- `docs/mcp-server.md`, `docs/comments.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `id` / face: parsed with `entity.ParseRef` and the same safe-segment
checks as HTTP (`isSafeStateRefSegment`), moved into `Ops`.
- anchor kind: allowlist of the three kinds via `Anchor.Validate`.
- body: `comments.ValidateBody` (size and content limits as HTTP).
- quote/prefix/suffix: located in the entity body server-side; never
trusted as offsets.
- comment id: opaque; looked up within the target thread only.

**Security-Sensitive Operations:**

- Authorization: read floor then `comment:*`, exactly as HTTP, in one
shared implementation.
- Existence oracle: unreadable and absent targets return the same error.
- Author spoofing: no author argument; author is the ctx principal.
- Read-only instance: write tools refused before any other gate.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1, AC2, AC4: `tools_comments_test.go` against a memstore project with
`comments:` enabled and memcomments, via the in-process MCP client the existing
tool tests use.
- AC3, AC4, AC5: same, with a declarative acl.yaml and two principals
(reader-only, commenter, other commenter).
- AC6: `tools/list` golden with and without `comments:`; one type
excluded via `comments.on`.
- AC7: server built over `acl.ReadOnlyACL`.
- AC9: accept on an open suggestion, on a resolved one, on one without a
  suggestion, on a stale body, and as a principal without entity update.
- AC8: existing `internal/dataentry` comment tests unchanged; plus
`ops_test.go` unit tests per error branch.
- Integration: `http_test.go`-style test over the remote MCP HTTP handler
with a JWT principal, add then list.
- Manual: local rela-server with `-mcp`, add a comment via MCP, see it in
the data-entry UI.

**Edge Cases:**

- Text quote occurring twice: prefix/suffix select; without them the first
occurrence (documented HTTP behaviour).
- Empty body, whitespace body, oversized body: rejected by ValidateBody.
- Comment id from another target: not found.
- Entity renamed between list and update: not found (thread moved).
- Bare id on faceless type: implicit face, no face_required.
- `resolved` only, `body` only, neither: neither is a no-op success, as
HTTP.

**Negative Tests:**

- Unknown anchor kind, property anchor with empty ref, text anchor without
quote, replacement on a non-text anchor.
- Unreadable target vs absent target: identical error text.
- Stdio: the four write tools are absent from `tools/list`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Refactoring the HTTP handler could change status codes or titles.
Mitigation: keep the mapping in the handler, existing tests unchanged, add table
tests for the mapping.
- Arch-lint: `comments` may not import `visibility`/`entity` resolution.
Mitigation: `Ops` depends only on call-site interfaces; if still blocked, place
it in `internal/comments/commentops`.
- Golden churn in `tools_list.golden.json`. Mitigation: regenerate with
the existing update flag and review the diff.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/mcp-server.md: the five tools, arguments and errors
- [x] docs/comments.md: MCP surface next to the HTTP surface
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** self-review on the plan; findings folded into the
sections above:

- significant: stdio runs as the system principal, not a signed-in user.
Addressed: explicit out-of-scope; stdio registers only `list_comments`
(Scope, Negative Tests).
- significant: duplicating the gates in MCP would split security-relevant
logic. Addressed: shared `Ops` (Approach 1-2).
- minor: caller-supplied type could mismatch the entity. Addressed: type
derived from the resolved entity (Approach 3).
