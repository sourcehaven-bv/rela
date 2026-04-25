---
id: PLAN-TFQC4
type: planning-checklist
title: 'Planning: Data-entry create form: prefix picker for multi-prefix types and manual ID field'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- `entitymanager.CreateOptions`: add `Prefix string` field (RR-8D8X3).
- `workspace/manager.go wsEntityManager.CreateEntity`: forward `opts.Prefix` to `workspace.CreateOptions.Prefix` (RR-8D8X3).
- `V1EntityType` JSON: expose full `id_prefixes` list (not just first).
- `handleV1CreateEntity`: accept optional `prefix` field in request body; validate it (see Security Considerations); reject `id` for non-manual types (RR-BJW16).
- `frontend/src/types/schema.ts`: add `id_prefixes?: string[]` to `EntityType`.
- `InlineCreateModal.vue`: show prefix picker when `id_type !== 'manual' && id_prefixes.length > 1` (RR-2R1HG).
- `DynamicForm.vue`: show manual-ID text field (create mode only) when `id_type === 'manual'`; show prefix picker when `id_type !== 'manual' && id_prefixes.length > 1` (RR-6GPR4, RR-2R1HG).
- Unit tests for the Go handler + Vue components.
- One E2E test covering manual ID and multi-prefix flows (requires extending the e2e fixture metamodel).

Out of scope:
- Server-side rendering / HTMX legacy templates in `internal/dataentry/templates/` — v2 Vue SPA is the only path.
- Changes to `workspace.Workspace.GenerateID` (already supports prefix override).
- Manual-ID validation beyond "required, non-empty" (pattern validation is a separate feature via custom types; surfacing the metamodel-defined pattern as a form hint is deferred — see RR-M1TIC).
- Changing how existing single-prefix types render (must stay the same).
- Rename-in-edit-mode for manual-ID types (covered by FEAT-016; edit form shows ID as read-only display, no input, no rename support here — RR-6GPR4).

**Acceptance Criteria:**

1. Multi-prefix, non-manual types show picker in `DynamicForm.vue` and `InlineCreateModal.vue`.
   - Test: e2e metamodel has a type with `id_prefixes: ["A-", "B-"]`; the form shows a `<select>` with two options; selecting "B-" then submitting creates an ID starting with `B-`.
2. Single-prefix types do NOT show a picker.
   - Test: unit/component test against the existing `ticket` type (`id_prefix: "TKT-"`) asserts the picker element is absent.
3. Manual-ID types: show editable ID field in CREATE mode; show read-only ID display in EDIT mode; never show prefix picker (RR-6GPR4, RR-2R1HG).
   - Test: e2e visits a create form whose entity type has `id_type: manual`, types "custom-123", submits, verifies entity is created at `/entity/<type>/custom-123`. Component test for `DynamicForm` in edit mode asserts ID is rendered as `<div class="id-display">` or similar (not `<input>`).
4. Backend exposes `id_prefixes`; single-prefix types still expose `id_prefix` for back-compat (RR-M0LIU).
   - Test: `TestV1Schema_MultiPrefix` asserts `id_prefixes: ["A-", "B-"]` for multi-prefix type; `TestV1Schema_SinglePrefix_Compat` asserts a single-prefix type returns BOTH `id_prefix: "TKT-"` AND `id_prefixes: ["TKT-"]`.
5. Backend create handler validates `prefix` and `id` against entity type (RR-3GURO, RR-BJW16).
   - Test: `TestV1CreateEntity_PrefixOverride` posts `{prefix: "B-"}` against a multi-prefix type, asserts response ID starts with `B-`.
   - Test: `TestV1CreateEntity_UnknownPrefix` posts `{prefix: "UNKNOWN-"}`, asserts 422 with message listing allowed prefixes.
   - Test: `TestV1CreateEntity_IDRejectedForNonManual` posts `{id: "TKT-HACKED"}` against a short-ID type, asserts 422.
   - Test: `TestV1CreateEntity_EmptyPrefixUsesFirst` posts without `prefix`, asserts ID uses first prefix.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- No external library — this is project-specific form UX.
- `workspace.Workspace.GenerateID(entityType, prefix)` (`internal/workspace/workspace.go:675-698`) accepts a prefix override; `workspace.CreateOptions.Prefix` exists and is plumbed through `createEntity` → `createEntityCore`. But no caller currently sets it — it's dead code today.
- `entitymanager.CreateOptions` (`internal/entitymanager/entitymanager.go:20-27`) does NOT have `Prefix`; the workspace adapter (`internal/workspace/manager.go:31-36`) does NOT forward it. **Both must be added** (RR-8D8X3).
- `metamodel.EntityDef.GetIDPrefixes()` (`internal/metamodel/entity_def.go:134`) returns the normalized list of prefixes. `EntityDef.IsManualID()` (`:122`) is the gate for manual ID.
- `InlineCreateModal.vue:30-32, 121-123, 156-167` already handles `id_type === 'manual'`: conditional `<input>` + sets `payload.id`. Pattern reusable in `DynamicForm.vue`.
- `InlineCreateModal.vue` is invoked from `RelationPicker.vue` (inline-create-from-picker flow). The prefix picker makes sense there too — the user is creating a new target entity and still needs to pick which prefix (RR-KY4RI).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Backend (Go):

1. `internal/entitymanager/entitymanager.go`: add `Prefix string` to `CreateOptions`, documented as "Overrides default ID prefix (ignored when ID is set or type is manual)".
2. `internal/workspace/manager.go` `wsEntityManager.CreateEntity`: set `createOpts.Prefix = opts.Prefix`.
3. `internal/dataentry/api_v1.go`:
   - Add `IDPrefixes []string \`json:"id_prefixes,omitempty"\``to`V1EntityType`; keep `IDPrefix` (populated with first prefix for compat).
   - In `handleV1Schema`, populate `et.IDPrefixes = def.GetIDPrefixes()` always.
   - In `handleV1CreateEntity`:
     - Decode `req.Prefix string \`json:"prefix,omitempty"\``.
     - Look up `entityDef, ok := s.Meta.GetEntityDef(typeName)` (already in scope via `a.State()`).
     - Validation (RR-3GURO, RR-BJW16), BEFORE calling entityManager.CreateEntity:
       - If `req.ID != ""` and `!entityDef.IsManualID()`: 422 `"id not accepted for non-manual ID type; use 'prefix' instead"`.
       - If `req.Prefix != ""`:
         - If `entityDef.IsManualID()`: 422 `"prefix not applicable to manual ID type"`.
         - If `req.Prefix` not in `entityDef.GetIDPrefixes()`: 422 `"prefix '<x>' is not valid for type <type>; allowed: [...]"`.
     - Pass `entitymanager.CreateOptions{ID: req.ID, Prefix: req.Prefix}`.
4. `internal/dataentry/handlers_api.go` (legacy `handleAPICreateEntity` at `:359`): apply the same `req.ID` rejection for non-manual types for consistency (already-existing pre-existing risk; mention in commit, small blast radius).

Frontend (TS/Vue):

1. `frontend/src/types/schema.ts`: add `id_prefixes?: string[]` to `EntityType`.
2. `frontend/src/types/entity.ts`: add `prefix?: string` to `CreateEntity`.
3. Extract shared composable `useEntityIDControls(entityType, mode)` in `frontend/src/composables/useEntityIDControls.ts`:
   - Returns `{ showManualIDInput, showPrefixPicker, prefixOptions, manualId, selectedPrefix, buildPayloadFields() }`.
   - `showManualIDInput = mode === 'create' && entityType.id_type === 'manual'`.
   - `showPrefixPicker = mode === 'create' && entityType.id_type !== 'manual' && prefixOptions.length > 1`.
   - `prefixOptions = entityType.id_prefixes ?? (entityType.id_prefix ? [entityType.id_prefix] : [])`.
4. `InlineCreateModal.vue`: use the composable; replace existing manualId ref; render `<select>` under "Prefix" label when `showPrefixPicker`.
5. `DynamicForm.vue`: use the composable; in create mode render manual-ID input OR prefix picker; in edit mode for manual-ID types render a read-only `<div class="id-display">{{ props.entityId }}</div>` under the "ID" label, with a small helper note "IDs cannot be changed here; use rename." (RR-6GPR4).
6. API wiring: `entitiesStore.create` already accepts the payload; just pass `prefix` and `id` through.

Alternatives considered:
- Radio buttons vs `<select>`: `<select>` matches existing enum-field UX, less visual noise. Rejected radio.
- Letting the client compute the generated ID and sending the full ID: violates single-source-of-truth. Rejected.
- Adding prefix validation to `workspace.GenerateID` instead of the handler: would leak workspace errors with less context. Handler-level validation gives clearer 422 messages. Rejected.
- Duplicating the computed logic in each component (not extracting a composable): would diverge over time. Rejected.

**Files to modify:**

- `internal/entitymanager/entitymanager.go` (add Prefix to CreateOptions)
- `internal/workspace/manager.go` (forward Prefix)
- `internal/dataentry/api_v1.go` (V1EntityType + handleV1CreateEntity validation)
- `internal/dataentry/api_v1_test.go` (new tests)
- `internal/dataentry/handlers_api.go` (apply same id-rejection rule)
- `frontend/src/types/schema.ts` (add id_prefixes)
- `frontend/src/types/entity.ts` (add prefix? to CreateEntity)
- `frontend/src/composables/useEntityIDControls.ts` (NEW)
- `frontend/src/components/forms/InlineCreateModal.vue` (use composable, prefix picker)
- `frontend/src/components/forms/DynamicForm.vue` (use composable, manual ID + prefix picker)
- `frontend/src/components/forms/__tests__/InlineCreateModal.test.ts` (new or extend)
- `frontend/src/components/forms/__tests__/DynamicForm.test.ts` (new or extend)
- `prototypes/data-entry/project/metamodel.yaml` (add manual-ID and multi-prefix type for E2E)
- `prototypes/data-entry/project/data-entry.yaml` (add forms for the new types)
- `frontend/e2e/forms.spec.ts` (new tests)
- `docs/metamodel.md` (note about multi-prefix picker in data-entry UI)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `req.prefix`: user-submitted string. **Allowlist validation in the HTTP handler** against `entityDef.GetIDPrefixes()` (RR-3GURO). Empty string is allowed and falls back to the first prefix. For manual-ID types, any non-empty prefix is a 422. Unknown prefix is a 422 with a specific message listing allowed prefixes.
- `req.id`: user-submitted string. **Only accepted when `entityDef.IsManualID()`** (RR-BJW16). For non-manual types, any non-empty `id` is a 422 `"id not accepted for non-manual ID type"`. For manual types, workspace's duplicate check (`workspace.go:737`) and any custom-type pattern validation take over. Empty ID for manual types remains 422 from workspace.
- Schema fetch/entity create race: acceptable. If the metamodel changes between schema fetch and create, the user sees a clean 422 on submit. No defensive fetch needed (RR-6HR8S).

**Security-Sensitive Operations:**

- Entity file writes: already handled by workspace under `a.writeMu`. No new file-system surface.
- No auth changes; same-origin local server — existing `FEAT-ESLP` posture still applies.
- Prefix/ID allowlist prevents writing entities outside the schema's declared prefixes.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|----|------|
| 1  | E2E: multi-prefix type in project fixture, form shows `<select>`, submit with prefix "B-", assert created ID starts with `B-`. |
| 2  | Component test: render `InlineCreateModal` with a single-prefix type, assert prefix `<select>` is absent. |
| 3  | E2E: manual-ID type form, fill ID, submit, redirected to entity page. Component test for `DynamicForm` in edit mode asserts ID is read-only `<div>`, not input. |
| 4  | Go: `TestV1Schema_MultiPrefix` + `TestV1Schema_SinglePrefix_Compat` (RR-M0LIU). |
| 5  | Go: `TestV1CreateEntity_PrefixOverride`, `TestV1CreateEntity_UnknownPrefix`, `TestV1CreateEntity_IDRejectedForNonManual`, `TestV1CreateEntity_EmptyPrefixUsesFirst`, `TestV1CreateEntity_ManualIDEmpty`, `TestV1CreateEntity_ManualTypeRejectsPrefix`. |

**Edge Cases:**

- Empty `id_prefixes` at metamodel level (caught at metamodel load; not a runtime concern).
- Manual-ID types that ALSO declare `id_prefixes`: prefix picker must NOT render (RR-2R1HG); backend rejects `prefix` (RR-BJW16).
- Whitespace-only manual ID: browser `required` + server check; stays 422.
- Empty `prefix` for multi-prefix types: fall back to first prefix (legacy-client compatible).
- Unknown prefix: 422 listing allowed prefixes.
- Edit-mode for manual-ID types: read-only display, no input, no rename (RR-6GPR4).
- Race: metamodel changed on disk → stale select options → submit → clean 422 (RR-6HR8S).

**Negative Tests:**

- Go: `TestV1CreateEntity_UnknownPrefix` — POST `{prefix: "UNKNOWN-"}` → 422.
- Go: `TestV1CreateEntity_IDRejectedForNonManual` — POST `{id: "TKT-X"}` for short-ID type → 422.
- Go: `TestV1CreateEntity_ManualTypeRejectsPrefix` — POST `{prefix: "X-"}` for manual-ID type → 422.
- Go: `TestV1CreateEntity_ManualIDEmpty` — POST without `id` for manual-ID type → 422 from workspace.
- Component: submitting manual-ID form with empty ID is blocked by `required` attribute.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|------|------------|
| Breaking existing single-prefix forms. | Picker only renders when `> 1` prefix AND non-manual. All existing fixtures keep singular `id_prefix`; back-compat test verifies both JSON fields populated (RR-M0LIU). |
| Data-entry E2E fixture requires adding a manual-ID and multi-prefix type; risk of breaking unrelated e2e tests that depend on the fixture shape. | Add new types alongside existing `ticket`/`category`/`label`; don't modify them. |
| Frontend reactivity: selectedPrefix default must re-initialize when the modal reopens for a different type. | Extracted composable resets state on `onMounted` / watch on entityType; unit-tested. |
| `handleAPICreateEntity` (legacy endpoint) pre-existing issue of accepting `id` for non-manual types. | Fix in same PR for consistency; small behaviour change, documented in commit message. |
| Adding `Prefix` to `entitymanager.CreateOptions` breaks other callers that construct zero-value. | Zero value is "no override" → same as today. No call sites need changes except the two we update. Compiler would catch any issue. |

Effort: **m** (~1 day backend incl. entitymanager plumbing + ~1 day frontend
incl. composable extraction + tests).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — `docs/metamodel.md` add a short note that the data-entry UI now supports picking among declared prefixes.
- [x] ~~CLI help text~~ (N/A: no CLI changes)
- [x] ~~CLAUDE.md~~ (N/A: no new patterns worth noting at project level)
- [x] ~~README.md~~ (N/A: no README-level changes)
- [x] API docs — `id_prefixes` and `prefix` request field are new schema surface; verify OpenAPI (if auto-generated) reflects them during implementation.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- RR-8D8X3 (critical, addressed): entitymanager.CreateOptions.Prefix added + forwarded in wsEntityManager.
- RR-3GURO (critical, addressed): explicit allowlist validation in `handleV1CreateEntity` with specific 422 message.
- RR-BJW16 (significant, addressed): `req.id` rejected for non-manual types; `req.prefix` rejected for manual types.
- RR-6GPR4 (significant, addressed): edit-mode renders read-only ID display; no rename implied.
- RR-M1TIC (minor, deferred): ID pattern hint deferred as separate work; not blocking here. Noted in scope.
- RR-6HR8S (minor, addressed): stale-prefix handling documented as "accept clean 422 on submit".
- RR-2R1HG (minor, addressed): picker suppressed for manual-ID types in AC 3 and scope.
- RR-M0LIU (minor, addressed): explicit back-compat test added.
- RR-KY4RI (nit, addressed): InlineCreateModal invocation via RelationPicker confirmed; picker UX applies uniformly.
