---
id: PLAN-W4TADJ
type: planning-checklist
title: 'Planning: Wire a policy-backed FieldWriteGate so MCP and Lua callers cannot write fields their policy hides'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** `entitymanager.FieldWriteGate` is wired as `AllowAllFieldGate{}`
everywhere, so MCP (rela-server) and Lua writes ignore `fields:`/`options:`
grants that the dataentry HTTP path enforces. The sync stack (FEAT-XYQMUB)
writes through Lua, so it must not be able to set fields its principal may not.

**In scope.**
- Shared field-write check in `internal/affordances` (moved from dataentry, same rule names and reasons).
- A policy-backed gate wired from appbuild.
- The gate on `CreateEntity` as well as `PatchEntity`. dataentry gates create (BUG-Q60V); without it a hidden field could be set through MCP/Lua create.
- CLI opt-out.
- dataentry maps a manager-returned denial to its 403.

**Out of scope (deferred).**
- Migrating dataentry PATCH onto the shared gate only.
- Dropping dataentry's own resolver.
- Hoisting `storeRelationLookup` and its fail-open on iteration errors.

These are refactors that do not change who may write what. They get their own
follow-up ticket.

**Acceptance.**
1. With a policy, MCP/Lua `update_entity` setting or unsetting a hidden or read-only field is refused with `field-affordance:hidden|read-only:<field>`. A filtered enum option is refused with `field-affordance:enum-filtered:<field>=<opt>`. An undeclared field is refused as hidden.
2. MCP/Lua `create_entity` with such a field is refused the same way.
3. The CLI (`rela update`, `rela create`) still writes any field. `rela scheduler` is gated.
4. Automation output (cascade writer) and elevated writes are not field-gated. The three TKT-80EWGM constraint tests pass unmodified.
5. dataentry denial responses are unchanged, and the existing affordance tests pass.
6. Without a policy, or with no affordance grants, the gate is permissive. A policy that fails to compile refuses construction.
7. `just arch-lint` passes.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the seam and design were settled in TKT-80EWGM)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal policy seam)
- [x] Checked codebase for similar patterns or reusable code: `validateFieldWrite` (dataentry/affordances.go:468) and `buildFieldRedactor` (appbuild.go:1094) are reused.
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal seam)
- [x] Reviewed relevant rela concepts for prior art: TKT-80EWGM, TKT-BUYEW1, BUG-Q60V, RR-32XA5V, RR-BA1NIV, RR-00ERM9

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

1. **`internal/affordances/fieldwrite.go`:**
   - Rule constants with the existing strings.
   - `FieldWriteDenial{Rule, Path, Reason, Attribution}` implementing `error`, with `RuleID()`.
   - `CheckFieldWrite(v FieldVerdicts, declared map[string]bool, set, unset) *FieldWriteDenial`: the dataentry logic, moved, with set keys checked in sorted order so the first denial is deterministic.
   - `DeclaredProperties(meta, type)`.
   - `WriteGate` over `*PolicyResolver`, built with `NewWriteGate` (rejects nil). Its `CheckFieldWrite(ctx, e, set, unset) error` returns nil or a `*FieldWriteDenial`, never a typed nil.
2. **dataentry:** `validateFieldWrite` converts its verdicts (identical struct) and calls `affordances.CheckFieldWrite`, then maps the result to `AffordanceDenialError`. The constants stay; equality with the affordances strings is pinned by a test. The create and PATCH handlers map a `*affordances.FieldWriteDenial` returned by the manager to the same 403.
3. **entitymanager:**
   - New helper `fieldGated()`, true when the handle is neither elevated nor a cascade writer.
   - PatchEntity uses it.
   - CreateEntity gates the caller's properties after `authorizeAndAudit` and before `createCore`, so template defaults and automation output are not checked. The candidate is `{Type, Face: opts.Face, Properties}`.
4. **appbuild:**
   - `buildFieldPolicy` returns the redactor and the gate from ONE resolver, built fully before either escapes.
   - `buildFieldRedactor` stays as a thin wrapper for its other callers.
   - The permissive path gives `NopRedactor` plus `AllowAllFieldGate`.
   - New `Config.UngatedFieldWrites` / `WithUngatedFieldWrites()` select `AllowAllFieldGate`, documented as the operator trust boundary.
5. **CLI (`kong.go`):** passes `WithUngatedFieldWrites()` for every command except `scheduler`, whose writes are made for a configured principal. `rela mcp` already wires `NopACL`, so nothing changes there.

**Alternatives rejected.**
- Infer the CLI exemption from `principal.ToolCLI`. Rejected: allow-all must never be inferred from identity.
- Gate create on the merged entity (with template defaults). Rejected: operators' template defaults would be refused for principals that cannot write those fields. dataentry gates only the request's properties.

## Security Considerations

- [x] Input sources identified: the property keys and values in MCP/Lua/HTTP write requests, which are caller-controlled.
- [x] Input validation approach defined: allowlist. Only declared or resolver-known fields pass, and an undeclared key is refused as hidden (F8).
- [x] Security-sensitive operations identified: the field-level write authorization itself.
- [x] Error handling doesn't leak sensitive information: the same rule and reason text as dataentry. The gate runs only after row authorization (RR-32XA5V).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

- **affordances unit tests:**
  - hidden, read-only, enum (scalar and list), undeclared, resolver-known undeclared, unset, and the sorted first denial.
  - `WriteGate` returns an untyped nil when there is no denial.
- **entitymanager:** create is gated; the cascade writer and elevation are not gated on create; the existing patch tests pass unmodified.
- **appbuild:**
  - With an acl.yaml that has `fields:` grants, `EntityManager().PatchEntity` and `CreateEntity` as the restricted principal are refused.
  - `WithUngatedFieldWrites` allows the same write.
  - Without grants, the write is allowed.
- **Lua through appbuild:** `rela.update_entity` setting a read-only field raises with the rule.
- **dataentry:** the existing affordance tests, plus a rule-string equality test.

## Risk Assessment

- [x] Technical risks assessed with mitigations
  - **Double check on dataentry PATCH:** the same policy is checked twice, so the verdicts agree. The cost is one extra `FieldVerdicts` call per PATCH, which is acceptable.
  - **Other PatchEntity callers become gated** (comments, webhooks, caldav). They act for a request principal, which is the intended behaviour. The existing tests are the net.
- [x] Security risks assessed: the change only narrows access, and construction fails closed.
- [x] Effort estimated: m

## Documentation Planning

- [x] User-facing docs identified: docs/acl-security.md gets a short "field grants hold on every write surface" note.
- [x] Docs-checklist will be created when entering implementation
- [x] ~~docs/metamodel.md~~ (N/A)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] ~~docs/data-entry.md~~ (N/A)
- [x] ~~CLAUDE.md~~ (N/A: follows existing rules)
- [x] ~~README.md~~ (N/A)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: acl-security.md is updated)

## Design Review

- [x] Run `/design-review` before starting implementation: self-review done against the TKT-80EWGM constraints; no open findings
- [x] All critical/significant findings addressed in plan
