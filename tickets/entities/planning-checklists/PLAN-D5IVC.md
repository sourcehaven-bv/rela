---
id: PLAN-D5IVC
type: planning-checklist
title: 'Planning: Configurable list actions with keyboard shortcuts for bulk property updates'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- Extend existing `Action` type with `label`, `key`, `confirm`, `set` fields
- Add `actions` (list of action IDs) to `List` struct
- `Space` key for row selection (multi-select) in list views
- Configurable action keys that apply mutations to selected rows
- Action bar UI showing selection count and available actions
- Both `set` and `script` actions work with multi-select (invoke once per entity)
- `{{today}}` interpolation in set values
- Config validation for new action fields

OUT of scope:
- Batch mode (passing all selected IDs to a single script invocation) — future
- Actions on views/kanbans (list-only for now)
- Undo/toast feedback beyond success/error toasts

**Acceptance Criteria:**

1. Existing `actions` config extended with `label`, `key`, `confirm`, `set` — backward compatible
2. Lists reference actions by ID via `actions: [mark-done, archive]`
3. Space toggles row selection in list view; Escape clears selection
4. Pressing an action key with rows selected applies the action to all selected entities
5. `set` actions → PATCH per entity; `script` actions → POST per entity with entity context
6. Action keys do nothing when no rows are selected
7. When `confirm: true`, a confirmation dialog appears before applying
8. Action bar at bottom shows "N selected" and available actions with key hints
9. `{{today}}` in set values is interpolated to current date (YYYY-MM-DD)
10. Config validation: single char key, no reserved key conflicts, no duplicates per list, set XOR script, properties exist in metamodel

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- Existing `Action` type (`config.go:82-90`) has `script`, `params`, `description`. We extend it with `label`, `key`, `confirm`, `set`.
- `useListKeyboard.ts` handles j/k/Enter/e/n/Delete/h/l. Reserved keys for validation.
- `PATCH /api/v1/{plural}/{id}` handles property updates with ETag locking. Reuse for `set`.
- `POST /api/v1/_action/{id}` handles Lua script execution. Will accept entity context in request body.
- Navigation entries already support `action: action-id` for sidebar buttons — same reference pattern.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Backend (Go)

1. Extend `Action` struct in `internal/dataentryconfig/config.go`:
   ```go
   type Action struct {
       Description string            `yaml:"description,omitempty" json:"description,omitempty"`
       Script      string            `yaml:"script,omitempty" json:"script,omitempty"`
       Params      map[string]string `yaml:"params,omitempty" json:"params,omitempty"`
       Label       string            `yaml:"label,omitempty" json:"label,omitempty"`
       Key         string            `yaml:"key,omitempty" json:"key,omitempty"`
       Confirm     bool              `yaml:"confirm,omitempty" json:"confirm,omitempty"`
       Set         map[string]string `yaml:"set,omitempty" json:"set,omitempty"`
   }
   ```

2. Add `Actions []string` field to `List` struct (references action IDs)

3. Add `Actions map[string]Action` to `V1Config` struct, populate in `handleV1Config`

4. Rewrite `validateActions` in `validate.go`:
   - Action must have `set` or `script` (at least one)
   - Action must NOT have both `set` and `script`
   - When `script`: validate `.lua` extension, local path
   - When referenced by a list: `key` and `label` required
   - `key`: single char `[a-z0-9]`, not reserved (j/k/o/e/n/h/l)
   - No duplicate keys within a single list's referenced actions
   - Properties in `set` validated against referencing list's entity type

5. Extend `handleV1Action` to accept optional entity context in POST body:
   ```json
   {"entity_id": "TKT-001", "entity_type": "ticket"}
   ```
When present, `actionScriptContext.GetEntity()` returns the entity.

### Frontend (Vue/TypeScript)

1. Update types in `src/types/config.ts`: extend Action, add `actions: string[]` to List

2. Create `src/composables/useListSelection.ts`:
   - `selectedIds: Ref<Set<string>>` — selected entity IDs
   - `toggle(id)`, `clear()`, `isSelected(id)`, `selectAll(ids)`

3. Create `src/composables/useListActions.ts`:
   - Resolves action IDs to action configs from schema store
   - Registers keydown handler for action keys (only when selection non-empty)
   - For `set`: PATCH per entity; for `script`: POST per entity with entity context
   - Handles confirm flow, `{{today}}` interpolation, error toasts
   - Disables action keys while processing

4. Extend `useListKeyboard.ts`:
   - `Space` toggles selection on focused row
   - `Escape` clears selection (when selection non-empty)

5. New `ActionBar.vue` component:
   - Fixed bar at bottom, visible when selection non-empty
   - Shows "N selected" + action buttons with key hints
   - Click also triggers action

6. Update `EntityList.vue`: integrate selection + actions + action bar

**Files to modify:**

Backend:
- `internal/dataentryconfig/config.go` — extend Action, add Actions to List
- `internal/dataentryconfig/validate.go` — rewrite validateActions, add list action ref validation
- `internal/dataentryconfig/validate_test.go` — tests
- `internal/dataentry/api_v1.go` — add Actions to V1Config, extend action endpoint for entity context
- `internal/dataentry/api_v1_test.go` — tests
- `internal/dataentry/actions.go` — accept entity context in POST body

Frontend:
- `frontend/src/types/config.ts` — extend Action and List types
- `frontend/src/composables/useListSelection.ts` — new
- `frontend/src/composables/useListActions.ts` — new
- `frontend/src/composables/useListKeyboard.ts` — add Space/Escape
- `frontend/src/components/lists/ActionBar.vue` — new
- `frontend/src/components/lists/EntityList.vue` — integrate

**Alternatives rejected:**
- Separate `ListAction` type: Creates two action concepts. Unified is simpler.
- Actions in metamodel: Mixes presentation with schema.
- Per-list inline action definitions: Not reusable across lists.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Config (data-entry.yaml): Validated at startup. Keys allowlisted to `[a-z0-9]`, properties checked against metamodel.
- Frontend action trigger: `set` uses existing PATCH (validates against metamodel, ETag locking). `script` uses existing action endpoint (Lua sandbox, 5s timeout, writeMu serialization).
- `{{today}}` interpolation: Only `today` supported. No arbitrary template execution.
- Entity context in action POST body: entity_id validated against graph, entity_type against metamodel.

**Security-Sensitive Operations:**
- All mutations go through existing validated handlers. No new attack surface.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| Criterion | Test |
|-----------|------|
| Config parsing | Unit: YAML with extended actions parses correctly |
| List references | Unit: List.Actions resolves to action configs |
| Space selection | E2E: Space toggles row selection |
| Action key applies set | E2E: Select rows, press key, verify property changed |
| Action key runs script | E2E: Select rows, press key, verify script invoked per entity |
| No selection = no-op | E2E: Press action key without selection, nothing happens |
| Confirm dialog | E2E: confirm:true shows modal, cancel aborts, confirm applies |
| Action bar | E2E: Selection shows bar; clearing hides it |
| {{today}} | Unit: interpolation replaces with current date |
| Validation | Unit: reserved keys, duplicates, set XOR script, unknown properties, unknown action refs |

**Edge Cases:**
- Page navigation clears selection
- PATCH 404 (entity deleted) → error toast, continue with rest
- ETag mismatch → error toast per failed entity
- Disable action keys while processing
- Bulk actions trigger automations per entity (documented, confirm flag gates heavy ones)

**Negative Tests:**
- key: "jj" → validation error (multi-char)
- key: "j" → validation error (reserved)
- Duplicate keys in same list → validation error
- Empty set + no script → validation error
- set + script together → validation error
- Unknown action ID in list → validation error
- Unknown property in set → validation error
- List action missing key → validation error
- List action missing label → validation error

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Backward compatibility: extending Action type is additive, existing configs work unchanged
- Script actions with entity context: endpoint extended with optional body, backward compatible

## Documentation Planning

- [x] User-facing docs identified
- [x] ~~Docs-checklist~~ (N/A: docs handled via CLAUDE.md update)

**Documentation Impact:**
- [x] CLAUDE.md (document unified action config with list references)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-HS8L7, RR-GVJ06, RR-KYF81, RR-U607K, RR-IPEN3 —
all addressed
