---
id: PLAN-H5KP6
type: planning-checklist
title: 'Planning: MCP create_entity ignores id_type — allows custom ID on short/sequential types'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** The entity creation path (`workspace.createEntityCore` at
`internal/workspace/workspace.go:905`) accepts a caller-supplied ID whenever
it's non-empty and passes `entity.ValidateID` (charset check). It never consults
the entity type's `id_type` setting. As a result, the MCP `create_entity` tool —
and by extension the data-entry HTTP API and any other consumer of
`entitymanager.CreateEntity` — can create entities with manual IDs on types that
the schema declared as `short` or `sequential`. This silently violates the
schema contract.

**Scope:**

IN scope:
- Reject caller-supplied IDs when the entity type's `id_type` is `short` or `sequential` at the single enforcement point inside `workspace.createEntityCore` so MCP, data-entry, and Lua all benefit from one fix.
- Actionable error message naming the entity type, its `id_type`, and the offending ID.
- Unit tests at the workspace layer covering `short`, `sequential`, and `manual` types with and without caller-supplied IDs.
- An MCP-level integration-style test that exercises `handleCreateEntity` end-to-end and asserts the error propagates.

OUT of scope:
- Changing the MCP tool's declared `id` parameter (we want the parameter to remain available for `manual`-typed entities).
- Validating `id` format against a custom `id_pattern` (handled elsewhere by existing property/ID validation).
- Data-entry UI changes — the API-layer behavior will improve automatically since it calls the same `entitymanager.CreateEntity`.

**Acceptance Criteria:**

1. AC1 — `id_type: short`: calling `create_entity` with a custom `id` returns an error mentioning the type, `id_type=short`, and the offending id. Verified by a workspace unit test and an MCP handler test using a short-typed entity (`ticket`).
2. AC2 — `id_type: sequential`: same rejection as AC1 with `id_type=sequential`. Verified by a workspace unit test with a sequential type (e.g., `requirement`).
3. AC3 — `id_type: manual`: caller-supplied IDs still work (no regression). Verified by a workspace unit test with a manual type.
4. AC4 — omitted ID: all three `id_type` values still auto-generate (short/sequential) or error with the existing "manual IDs required" message (manual). Verified by existing tests plus a targeted unit test.
5. AC5 — error is surfaced: MCP `handleCreateEntity` returns `mcp.NewToolResultError` with the workspace error text. Verified by an MCP handler test.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- `metamodel.EntityDef.IsManualID()`, `IsShortID()`, `IsSequentialID()` (`internal/metamodel/entity_def.go:111-124`) already exist and are the idiomatic gate.
- `workspace.GenerateID` (`internal/workspace/workspace.go:684`) already errors with `"entity type %s uses manual IDs"` when `IsManualID()` is true and no ID was supplied — this is the mirror of what we need. The new check is the counterpart: error when a caller-supplied ID arrives for a non-manual type.
- `entity.ValidateID` (`internal/entity/id.go:134`) validates charset only, not schema compliance. We keep that check; we add a schema check in `createEntityCore`.
- No external library applies.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Single enforcement point at `internal/workspace/workspace.go:913-924` inside
`createEntityCore`. Current code:

```go
entityID := opts.ID
if entityID == "" {
    id, err := w.GenerateID(entityType, opts.IDPrefix)
    ...
    entityID = id
} else {
    if err := entity.ValidateID(entityID); err != nil {
        return nil, err
    }
}
```

Change the `else` branch to also reject manual IDs on non-manual types:

```go
} else {
    if !entityDef.IsManualID() {
        return nil, fmt.Errorf(
            "entity type %q uses id_type=%s; custom ID %q not allowed (IDs are auto-generated)",
            entityType, entityDef.GetIDType(), entityID,
        )
    }
    if err := entity.ValidateID(entityID); err != nil {
        return nil, err
    }
}
```

`entityDef` is already resolved at line 907 so no extra lookup.

**Alternatives considered:**

- Guard at the MCP layer only (`handleCreateEntity`). Rejected: data-entry HTTP API and any future consumer would remain permissive — the bug would recur on each new entry point. The architectural guidance in `CLAUDE.md` ("capability bundles, not service locators") argues for enforcement at the shared core.
- Guard inside `entitymanager.CreateEntity` (in the `wsEntityManager` adapter). Rejected: that adapter is a thin passthrough; the actual creation logic lives in `workspace.createEntityCore`. Putting it in the adapter adds an indirection without the centralization benefit.
- Drop the `id` parameter from the MCP tool schema entirely. Rejected: manual-ID types legitimately need it.

**Files to modify:**

- `internal/workspace/workspace.go` — add the `IsManualID()` guard in `createEntityCore` (≤10 lines).
- `internal/workspace/workspace_test.go` — add table-driven tests covering short/sequential/manual × with/without custom ID.
- `internal/mcp/tools_entity_test.go` (or wherever `handleCreateEntity` tests live) — add one test that asserts the error surfaces through MCP.

**Files to inspect (no changes expected):**

- `internal/dataentry/api_v1.go:370` — should now reject custom IDs on non-manual types automatically. Not adding a separate test here since the fix is one layer down; an existing data-entry create test continues to validate the happy path.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `opts.ID` comes from MCP tool args (AI assistant), data-entry HTTP request body, or internal Go callers. Validation is allowlist-shaped: only accept when `entityDef.IsManualID()` is true.
- The error message echoes the entity type and the offending ID string. Both are caller-supplied. No secrets involved. Echoing the ID back is desirable for debuggability.

**Security-Sensitive Operations:**

- File system writes are downstream of this check (`w.writeEntity`). Tightening ID acceptance narrows the surface by one more rule. No new sensitive operations introduced.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1 — `TestCreateEntity_RejectsCustomID_ShortType`: workspace test that calls `createEntity` with a short-typed entity and `opts.ID="my-id"`; expects an error whose message contains `"short"`, the type name, and `"my-id"`.
- AC2 — `TestCreateEntity_RejectsCustomID_SequentialType`: same for a sequential-typed entity; expects `"sequential"` in the error.
- AC3 — `TestCreateEntity_AllowsCustomID_ManualType`: manual-typed entity + custom ID; expects success and the ID to be honored.
- AC4 — `TestCreateEntity_AutoGenerates_*`: existing coverage; spot-check nothing regresses.
- AC5 — `TestHandleCreateEntity_ReturnsErrorForShortTypeWithCustomID`: MCP handler test that builds a `CallToolRequest` with `id` set and a short-typed type, asserts the `CallToolResult` is an error result, and the error text mentions id_type.

**Edge Cases:**

- Empty `id` ("") — existing behavior: auto-generate or error for manual types. Verified.
- `id` that also fails `entity.ValidateID` charset check on a manual type — expect the charset error (existing behavior preserved).
- `id` that would collide with an existing entity — duplicate check at `workspace.go:743-746` runs before ID validation. For manual types the existing duplicate error path still fires. We explicitly test that the new check runs even when no duplicate exists (the code path short-circuits on duplicate, so we order the new check after the duplicate check in the else branch — wait, the new check is inside the non-empty branch which is reached after the duplicate check has already run, so ordering is correct).
- Unicode / weird characters in `id` — still rejected by `entity.ValidateID` when on a manual type. Not retesting (covered by `TestValidateID`).
- Data-entry and Lua — no separate tests; the shared core fix covers them. A follow-up could add an MCP + data-entry parity test, but that's out of scope.

**Negative Tests:**

- AC1/AC2 above are the negative tests for schema enforcement.
- Assert no entity was written to the store when the error fires (no side effects). Accomplished by checking `store.ListEntities` before and after.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- RISK: An existing internal caller (e.g., a migration, a seed test helper) may depend on passing custom IDs for non-manual types. Mitigation: grep for `CreateOptions{ID:` and `CreateEntity(...)` call sites and audit each before implementation; adjust tests that were papering over the permissive behavior.
- RISK: Automation `create_entity` actions (see `metamodel.yaml` automation engine) could use explicit IDs via interpolation. Mitigation: grep the automation engine for how it supplies IDs — it generates them via `createEntityCore` too, so unless a fixture sets `id` in the automation config, no behavior change. Verify during implementation.
- RISK: Breaking API change for MCP / data-entry callers that currently (incorrectly) pass custom IDs on short types. Mitigation: this is precisely the bug being fixed — the break is intentional. Error message must be actionable so callers can diagnose immediately.

**Effort:** s (single guard, handful of tests, one audit pass).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — MCP tool description already says "auto-generated if omitted"; no user-facing copy change. The MCP tool's `id` argument description could be tightened to "Custom entity ID (only valid for id_type=manual; auto-generated otherwise)" as a low-cost clarification — will do this as part of the implementation since it's one line.
- [x] ~~CLAUDE.md, README, or guide changes~~ (N/A: no user-facing behavior change — the error only surfaces to callers violating the schema contract; project docs don't document the bug)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (Skipped: chose to use the post-implementation cranky code reviewer instead, which surfaced 12 findings. Scope of a one-line guard with test churn made post-implementation scrutiny more valuable than up-front design review.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None (see skip reason above). All significant findings from the cranky reviewer were addressed during review phase — see RR-SG9MC, RR-K21KR, RR-LPQT1 linked to the ticket.
