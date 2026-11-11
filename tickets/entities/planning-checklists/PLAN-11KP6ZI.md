---
id: PLAN-11KP6ZI
type: planning-checklist
title: 'Planning: Direct messages on channels: decide the DM model'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

This ticket is a design discussion. Its output is the DM model (recorded as a
decision) and four implementation tickets. Each of those gets its own planning
checklist with detailed tests and file lists.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: the model for 1:1 direct messages between people on top of realtime
channels; the split into implementation tickets.

Out: group DMs, agents as participants, operator read access (left open, not
built), sessions/triggers/promote in DMs, implementation itself.

**Acceptance Criteria:**

1. The model is recorded as an accepted decision linked to the ticket's
concepts. Test: the decision exists with context, consequences and date, and
`analyze validations` passes.
2. Each follow-up ticket has a description, acceptance criteria, effort and
dependencies. Test: the three analyze gates report nothing for them.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: options A/B/C are compared in the ticket body)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-tree channel subsystem is the base)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the in-tree `_flow-run` pattern was the user's chosen reference)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (options are in TKT-55RNWWD).

**Existing Solutions:**

- `_flow-run` managed type: `internal/metamodel/flowrun.go`,
`internal/metamodel/reserved.go`, `internal/acl/managed.go` (`FlowEngineRole`,
`wildcardCovers`).
- Channel anchor and key: `internal/channels/channels.go:241`.
- Role sources: `internal/realtimecfg/compiled.go:240`.
- Comments kept out of the graph: `.claude/rules/comments.md`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** see "Proposed design" in TKT-55RNWWD: a managed `_dm`
type with two `_member` edges to user entities, a hashed pair id, a
participants-only read route, a `system:realtime` writer role, and a managed
`_direct` channel type configured by a `direct:` block in `realtime.yaml`.

Rejected: an operator-declared conversation type (privacy depends on ACL
authoring), a principal-set anchor outside the graph (anchor-is-entity runs
through the whole channel subsystem; XL), anchoring on the user entity
(asymmetric).

**Files to modify:** listed per follow-up ticket. Main packages:
`internal/metamodel`, `internal/acl`, `internal/realtimecfg`,
`internal/channels`, `internal/appbuild`, `internal/dataentry`, `internal/mcp`,
`internal/cli`, `frontend/`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the target person of an open-DM call (must be a
user entity the caller can read, not the caller); `direct:` config (validated at
load, refused without person mapping); messages (existing channel validation).

**Security-Sensitive Operations:** the `_dm` read route is the whole privacy
guarantee. It is compiled as an allowlist at load (no runtime deny), lowers to a
store query, and gives the uniform 404 to non-participants so DM existence stays
secret. Only `system:realtime` writes `_dm`; acl.yaml cannot name it.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** per follow-up ticket. Core: a third person, an admin with
`"*"` read and a client with read-all ceiling all get 404 on a DM and its
channel; both participants read and post.

**Edge Cases:** DM with yourself; concurrent opens of the same pair; target user
entity deleted or renamed; person mapping removed after DMs exist; `direct:`
removed from realtime.yaml.

**Negative Tests:** acl.yaml naming `_dm` or `_member` is refused; schema.yaml
declaring `_dm` is refused; an agent opening a DM is refused.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A read path that skips the row gate leaks DM existence or content.
Mitigation: reuse the visibility wrappers; storetest coverage for the route.
- Search or list pages show `_dm` headers. Mitigation: exclude the type from
menus and lists; gate covers search.
- Effort: L across four tickets.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: created on the follow-up tickets)

**Documentation Impact:** realtime guide (`direct:` block), acl-security
(managed `_dm` read rule), data-entry guide (DM entry point), CLI and MCP
reference.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: runs in planning of the follow-up for the managed type and ACL route, which carries the security-sensitive part)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: see above)

**Design Review Findings:** N/A here.
