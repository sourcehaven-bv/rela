---
id: PLAN-5A32WS
type: planning-checklist
title: 'Planning: Comments over stdio MCP: add_comment and watch on list_comments'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

A coding agent runs as `rela mcp --run-as integration:runner`. People talk to it
through comments on its ticket in the web app. Today stdio MCP can list comments
but not add one, and `watch` cannot wait on `list_comments`, so the agent has to
poll.

**Scope:**

In scope:
- Stdio `add_comment`, registered only when `rela mcp` runs with `--run-as`.
The author is the job identity. Without `--run-as` the tool stays absent: which
person a plain local session comments as is still open (TKT-QIQO1W), and an
agent commenting as `$USER` would look like the human wrote it.
- `list_comments` as a `watch` target on both transports.

Out of scope:
- Stdio `update_comment`, `delete_comment`, `accept_comment_suggestion`.
- A comment event stream. Comments are not in the graph, so they emit no store
event; a poll covers in-process and cross-process changes alike.

**Acceptance Criteria:**
1. Stdio with `--run-as integration:runner`: `add_comment` on `TKT-1` stores a
comment whose author is `integration:runner`; `list_comments` returns it.
2. Stdio without `--run-as`: `add_comment` is not in the tool list.
3. `add_comment` on a faced entity by bare id fails with the ambiguity error;
with `ID@face` it succeeds. On a type without comments it answers
`comments_not_enabled`; on a missing id, not found.
4. A text anchor is resolved against the entity body as on HTTP (quote not
found is refused).
5. `watch` with a `list_comments` target returns `changed` within the poll
interval after a comment is added (in this process or another).
6. `watch` lists `list_comments` in its target enum only when the tool is
registered.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern exists)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `internal/cli/mcp_comments.go` `stdioComments` already serves stdio
`list_comments`; `add_comment` extends it.
- `internal/dataentry/comments_wiring.go:251` `buildTextAnchor` derives the
stored text anchor; it moves into `internal/comments` so both transports use one
copy.
- `internal/mcp/tools_watch.go` channel targets already replay on a poll
(`watchChannelPoll`, stdio `stdioChannelPoll`); comment targets join that poll.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
0. `internal/comments`: `Service.Add` refuses every reserved author today, so
no job identity can comment. Export `ValidAuthor(user)`: a person, or an
`integration:` name that `principal.ValidJobIdentity` accepts. `system:` names
(rela's own and operators') stay refused. This applies on both transports: a
remote MCP integration token can now comment too, under its `comment:add` grant.
1. `internal/mcp`: split `CommentWriter`. A new `CommentAdder` holds
`AddComment`; `CommentWriter` keeps update, delete and accept. `CommentDeps`
gains `Adder`. `add_comment` registers when `Adder` is set, the other three when
`Writer` is. The remote wiring sets both.
2. `internal/comments`: `BuildTextAnchor(content, quote, prefix, suffix)`
moved from dataentry, unchanged.
3. `internal/cli`: `stdioComments.AddComment` resolves the write target with
`GraphReader.WriteTarget` (bare id on a faced type is ambiguous), reads the
entity at that ref for its type and body, checks the type is commentable, builds
the anchor, and calls `comments.Service.Add`, which takes the author from the
ctx principal. `mcpServices` records the `--run-as` identity; `Adder` is wired
only when one was given and `comments.ValidAuthor` accepts it, so a `system:`
run-as identity does not list a tool that always refuses. No ACL gate, like
every other stdio write (NopACL; the filesystem is the trust boundary).
4. `internal/mcp` watch: `list_comments` joins `watchOrder`. A comment target
is marked polled; the poll ticker starts when any channel or comment target
exists, and `passPoll` replays comment targets too. A store event still triggers
a full pass (an entity edit can detach an anchor).

**Alternatives rejected:**
- Offer stdio `add_comment` without `--run-as`, authored as `$USER`: reopens
the TKT-QIQO1W decision; out of scope.
- An in-process comment notifier: misses posts from rela-server, which is the
case that matters. The poll is needed anyway.

**Files to modify:**
- `internal/mcp/tools_comments.go`, `internal/mcp/tools_watch.go`, tests
- `internal/comments/textresolve.go` (or new file), `internal/dataentry/comments_wiring.go`, `comments_ops.go`, `comments_mcp.go`
- `internal/cli/mcp_comments.go`, `internal/cli/mcp_wiring.go`, `internal/cli/mcp.go`, tests
- `docs/comments.md`, `docs/mcp-server.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Address: parsed by `WriteTarget`; an unparseable or missing id is not found.
- Body and anchor: `comments.Service.Add` validates body size and control
characters and the anchor; text anchors go through the moved `BuildTextAnchor`
with its quote limits.

**Security-Sensitive Operations:**
- Author attribution: taken from the ctx principal by `Service.Add`, never from
arguments. Only a validated `--run-as` job identity can author, so an agent
cannot comment as a person.
- No ACL gate on stdio, by the documented stdio trust model. The watch replay
of `list_comments` on the remote server goes through the same handler as the
tool, so it keeps the HTTP gates.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1, AC3, AC4: `internal/cli/mcp_comments_test.go` table test over a
project with a commentable type and a faced type.
- AC2: wiring test on `mcpServices.deps` with and without the run-as flag.
- AC5, AC6: `internal/mcp` watch tests with a fake comment reader whose result
changes; checks the poll wakes the target and the enum.
- Dataentry tests keep passing after the move (`BuildTextAnchor`).

**Edge Cases:**
- Empty body, oversized body: refused by `Service.Add`.
- Quote shorter than 5 runes or not in the body: refused.
- Entity deleted between watch passes: list answers not found; digest moves.

**Negative Tests:**
- Uncommentable type, missing id, bare id on faced type, `add_comment` absent
without `--run-as`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Poll cost: one comment read per comment target every poll interval (2s
remote, `stdioChannelPoll` on stdio). Bounded by 16 targets per watch and 32
watches per server, the same bound as channel targets.
- Splitting `CommentWriter` touches the remote wiring; covered by existing
remote MCP comment tests.

Effort: s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- `docs/comments.md` § The MCP tools
- `docs/mcp-server.md` comment tools and watch targets

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-YB41PM (significant, step 0), RR-0SERT0 (minor,
step 3), RR-MVS2MH (minor, risks)
