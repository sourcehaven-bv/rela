---
id: PLAN-9J578N4
type: planning-checklist
title: 'Planning: Trigger placeholder for the current user'
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the trigger placeholder `{{current_user.id}}` in every field that
interpolates entity placeholders. Out: inline Lua code (scripts have
`rela.principal`); a data-entry form default; the raw principal as a value.

**Acceptance Criteria:** see the ticket; each criterion names its test.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: one placeholder on an existing mechanism)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal mechanism)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Lua: `rela.principal.user` plus `create_relation` in a trigger script (TKT-5U6NRR). Works, but needs a script.
- Trigger `relate: {type, to}` with interpolation; an empty `to` skips the relation (compile.go renderOutput).
- `predicatefns.QueryIdentity` and `appbuild.queryIdentityFor`: the entity id is `principal.User` when `RawUser` is set and differs, which marks a `principal_property` substitution.
- Data-entry form defaults have no user source; a trigger runs for every write path.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `Engine.Process` resolves the acting user's entity id
once from ctx: a stamped `predicatefns.QueryIdentity` with an entity id wins;
else `principal.Stamped` with a substitution (`RawUser` set and different from
`User`); else empty. It travels on the `Event` copy (unexported field) into
`TemplateVars.CurrentUserID`, which `Interpolate` reads for `current_user.id`.
`InterpolateSafeOnly` does not. `knownPlaceholder` admits it.

Alternatives: a `default: current_user` on data-entry fields, rejected because
it covers only the form and not the API, MCP or CLI, and the relation would
still need a write; falling back to the raw principal, rejected because a raw
login is not an entity id and the relate would fail or point at a stranger
entity whose id happens to match; Lua only, kept as the documented route for
anything beyond one relation.

**Files to modify:** internal/flowcfg/triggers/{template.go,engine.go, parse.go}
and tests; GUIDE-flows.md (and the generated docs/flows.md).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the value comes from the authenticated principal
on ctx, never from request data. The placeholder name is an allowlist entry.

**Security-Sensitive Operations:** relation writes. They go through the existing
cascade path (`cascadeHost.WriteRelation`), so the metamodel checks and
write-path rules are unchanged. The value is never interpolated into Lua source.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria.

**Edge Cases:**
- No principal on ctx, the `unknown` default, a raw principal without substitution: empty.
- `RawUser` equal to `User`: empty (no substitution happened).
- A stamped query identity without an entity id: falls back to the principal.

**Negative Tests:** `{{current_user}}`, `{{current_user.name}}` and `{{
current_user.id }}` are load errors.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- A trigger fired by a background job or the CLI gets an empty value. Mitigation: documented; the relate is skipped.
- Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/flows.md (via GUIDE-flows): placeholder table and a note-author example, with a pointer to the Lua route.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: one placeholder; the code review covers it)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
