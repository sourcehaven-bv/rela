---
id: PLAN-YRQ5DG7
type: planning-checklist
title: 'Planning: List assign: defaults for create, drop and move'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `assign:` on lists and pages; action `assign: <list>`; `GET/POST
/api/v1/_assign`; SPA create prefill, sidebar drop targets, assign actions,
mismatch notice; example update. Out: drop on kanban columns, multi-target
assign, assign on calendars.

**Acceptance Criteria:**
1. A config with an unknown property, a computed property, an enum literal outside the values, a non-compiling expression, `related()` in an expression, an unknown relation, or `target` on a list fails to load (validator table test).
2. `GET /_assign?list=today` returns `{properties:{when:"today",start_date:"<today>"}}` (handler test).
3. `POST /_assign` moves rows the caller may update; a row the caller cannot update gets `ok:false` with a 403-class error and is unchanged; hidden rows answer as not found (ACL test).
4. `matches:false` when the written row still fails the list condition (handler test).
5. Page assign with `target` creates the relation to the dropped-on entity; a target the caller cannot read is a not-found (handler test).
6. Unnamed properties survive (PatchEntity), pinned by test.
7. SPA: drop on a sidebar entry calls POST and shows a notice on mismatch (vitest).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: design agreed in the gap decisions)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-house config pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Section prefill in `frontend/src/composables/useListGrouping.ts` and `EntityList.vue` `createPrefill`.
- `set:` actions in `useListActions.ts` (SPA PATCH per row).
- `configuredListRows` with `reach` (nav_status.go) answers membership for a batch of ids.
- `predicate.CompileValue` as used by `internal/computed`.
- `AuthorizeUpdate` + `PatchGranted(WithRelations)` in write_handler.go.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `dataentryconfig.Assign{EntityType, Properties
map[string]AssignValue, Relations map[string]string}`; `AssignValue` unmarshals
a scalar literal or `{expr: ...}`. Validation in `validate_assign.go`;
expressions compiled in `conditionlint` style at load (`assign` programs on
Schema). Handler `assign_handler.go`: resolve values once per request, then for
each address: `visibility.Resolver.WriteRow` → `AuthorizeUpdate` →
`PatchGrantedWithRelations` (relations replace the named type at the row).
Membership: one `configuredListRows` call with `reach` = written ids. Action
`assign: <list>` validated to name a list with an assign. Alternative rejected:
resolving values server-side and PATCHing from the SPA; it needs a second round
trip per row for membership, and splits the write rule across two places.

**Files to modify:** internal/dataentryconfig/{config.go,validate_assign.go},
internal/dataentry/{assign_handler.go,api_v1.go,router_walk_test.go}, frontend
list/sidebar/actions, docs-project data-entry docs, examples/todo-app.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Body `{list|page, target, entities[]}`:
list/page must name a configured entry with an assign (403/404 on config key is
fine, config is not secret); entities capped at 100 per request; each address
parsed like the PATCH path; target resolved through the gated reader, so a
hidden target is 404 indistinguishable from a missing one.

**Security-Sensitive Operations:** Writes: authorized per row by the manager
(`AuthorizeUpdate`), field and relation checks unchanged; values come from
config, never from the body. Membership answer is computed over the gated reader
for rows the caller just wrote, so it discloses nothing new. Expressions run in
the sandboxed predicate engine, no `related()`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria 1–7.

**Edge Cases:** empty entities; >100 entities (400); address with face; entity
of another type (error per row); expression evaluating to nil (property unset
via MetaUnset).

**Negative Tests:** invalid configs fail load; unauthorized row write refused;
hidden target 404; unknown list 404; oversize batch 400.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Replace semantics on a relation could drop tags if an operator
assigns a many-relation; documented, and assign replaces only the relation types
it names. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs-project data-entry list reference (assign, action
assign), API reference for `/_assign`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-5O2616W (relations go through the PATCH relation
join; many-relations add), RR-MR71Q1M (app clock via predicatefns.Bind),
RR-WT6YREM (per-row results, documented).
