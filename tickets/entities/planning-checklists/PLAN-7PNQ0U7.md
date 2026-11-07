---
id: PLAN-7PNQ0U7
type: planning-checklist
title: 'Planning: Scoped elevated updates for data-entry actions'
started: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: `allow_acl_bypass: write` plus `elevated_updates:` on a data-entry script action; `admin.update_entity` on the bypass handle for that action only; load-time validation; audit marker; docs.
Out: elevated create, content writes, external refs, elevation on the IdP webhook, elevation for `set:` actions.

**Acceptance Criteria:**
1. An elevated action updates a declared field on a row the caller may not update; `rela.update_entity` on the same row is denied.
2. An undeclared type or property, content, or an unreadable target is refused and nothing is written.
3. `admin.update_entity` is absent without the opt-in; no other admin method is present with it.
4. Audit rows carry the real principal, the bypass marker and `action:<id>`.
5. Config validation refuses every invalid combination.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: DEC-O59WM4 already fixes the mechanism)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal ACL feature)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal ACL feature)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `rela.bypass_acl` and the `admin` handle (internal/lua/elevation.go, TKT-D8T148, TKT-Y3JVFK).
- `Manager.elevated()` and `UpdateGrant` (internal/entitymanager/manager.go).
- Document elevation validation (`validateDocumentElevation`).
- DEC-O59WM4: one escalation mechanism, no inline `roles:`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `entitymanager.ScopedUpdates`: built from a Manager, a type-to-properties scope and a source label. Its `PatchEntity` checks the caller's read gate and the scope on the stored row, then patches through the elevated handle. Content and external refs are refused.
- `lua.WriteDeps.ElevatedUpdater`: registers `rela.bypass_acl` with only `admin.update_entity`.
- dataentry `actionOps.run` sets it for an action with `allow_acl_bypass: write`.

Alternatives rejected: inline `roles:` on actions (DEC-O59WM4 rejects it); an unscoped `admin.update_entity` (any script bug could write any field).

**Files to modify:**
internal/entitymanager/scoped_updates.go, internal/entitymanager/manager.go, internal/lua/deps.go, internal/lua/elevation.go, internal/lua/runtime.go, internal/dataentryconfig/config.go, internal/dataentryconfig/validate.go, internal/dataentry/actions_ops.go, internal/dataentry/write_handler.go, internal/dataentry/app.go, docs guides, tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `data-entry.yaml` (operator): validated at load against the metamodel; an allowlist of types and properties.
- Script arguments (operator script, caller-influenced values): each property name is checked against the allowlist; an unreadable target returns the uniform not-found before the scope check.

**Security-Sensitive Operations:**
- The elevated write. Protected by the action `permission:`, the scope allowlist, the caller read gate, transition guards, and an audit row with the real principal.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. dataentry end-to-end test with a Declarative ACL that lets the caller read but not update tickets.
2. entitymanager table test: wrong type, wrong property, MetaUnset outside scope, content, external refs, unreadable row, missing row.
3. lua table test: handle shape with and without the updater.
4. entitymanager and dataentry audit tests.
5. dataentryconfig table test.

**Edge Cases:**
Empty patch, `rela.sync.EMPTY` clears, handle used after the closure.

**Negative Tests:**
Each refusal leaves the stored row unchanged.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Elevation leaking into cascades: mitigated by `gated()` at dispatch.
- Scope bypass through MetaUnset: checked like properties.
Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
GUIDE-data-entry (action fields, elevated updates), GUIDE-lua-scripting (elevated access).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None. The design was checked against DEC-O59WM4 and internal/entitymanager/CLAUDE.md; rela-security-reviewer runs on the diff.
