<!-- @managed: claude-workflow v1 -->
---
id: PLAN-IHC7D
type: planning-checklist
title: 'Planning: View wire-shape — typed _props + _fields per cards/list row entity'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** see TKT-IHC7D ticket body. Prerequisite for TKT-IHC7C (cards/list inline edit). Pure backend wire-shape extension + frontend TS types; no behavioural change for existing consumers.

**Acceptance Criteria:**

1. **`V1ViewEntity` gains two optional fields** in `internal/dataentry/api_v1.go`:
   ```go
   Props            map[string]any                   `json:"_props,omitempty"`
   FieldAffordances *map[string]V1FieldAffordance    `json:"_fields,omitempty"`
   ```
   `_props` is a plain map with `omitempty` (RR-FD1E: presence/absence sufficient; no closed-world semantic). `_fields` uses the pointer-to-map idiom so "absent" vs "present-but-empty" stays distinguishable — matches `V1Entity.FieldAffordances` precedent. A Go doc comment on both fields documents the key-set invariant (RR-FD1B): `keys(_props) ∩ hidden(e) == ∅`, `keys(_fields) ∩ hidden(e) == ∅`, and `_fields` may have keys absent from `_props` when the property has no stored value but a non-default verdict.

2. **`SectionEntityData` carries both the typed properties AND the precomputed verdict** (RR-FD1E reverse of round 1's alt-b rejection — `buildSections` already takes ctx, so computing verdict there is cleaner than threading the entity to the wire converter):
   ```go
   Props  map[string]any                  // typed properties from e.Properties, hidden-stripped
   Fields map[string]V1FieldAffordance    // sparse per-cell affordance verdict
   ```
   The unexported `entity *entity.Entity` back-reference proposed in round 1 is DROPPED. Wire converter just reads the precomputed maps — clean package boundaries.

3. **Both non-entry branches in `buildSections` populate `Props` and `Fields`** (RR-FD1C — both `properties`/`list` at L192-219 AND `content`/`cards` at L279-308 need wiring). Extract a shared helper:
   ```go
   func (a *App) buildSectionEntityData(ctx context.Context, e *entity.Entity, secFields []ViewField, eDef *metamodel.EntityDef) SectionEntityData {
       sed := SectionEntityData{
           ID: e.ID, Title: ..., Type: e.Type, EditFormID: a.editFormForType(e.Type),
           Props:  a.copyVisibleProperties(ctx, e),       // hidden-stripped
           Fields: a.computeFieldAffordances(ctx, e),     // RR-FD1E: precomputed here
       }
       for _, f := range secFields { /* existing string-fields */ }
       return sed
   }
   ```
   Both branches call it. The string-field building stays inline within the helper because it's per-section-config, not per-entity.

   For the entry-source path (`sec.Source == "entry"`), `_props` is not populated because the entry-level surface already carries `properties` and `_fields` via the existing `V1Entity` shape.

4. **Wire conversion populates both fields at ALL `V1ViewEntity` construction sites** in `api_v1.go`. There are at least two sites (RR-FD2A — round 2 caught the second):
   - `api_v1.go:~3010` — top-level entities under `V1ViewSection.Entities`.
   - `api_v1.go:3066-3077` — group-card display under `GroupData.Entities`. Currently dormant (`buildSections` never populates `GroupData.Entities`), but applying the same snippet keeps parity if grouped cards ever ship.

   For each row entity, dumb-copy the precomputed maps off `SectionEntityData` (RR-FD1E):
   ```go
   v1Ent.Props = e.Props
   if e.Fields != nil {
       fields := e.Fields // local copy of map header for &-taking
       v1Ent.FieldAffordances = &fields
   }
   ```
   No entity back-reference, no `computeFieldAffordances` call at the wire layer — that work happened in `buildSections` where `ctx` lives naturally.

5. **Hidden-property stripping is INTRODUCED to the cards/list path by this ticket** (RR-FD1A correction — the round-1 framing claimed `stripHiddenProperties` already covered this, which is wrong. `V1ViewEntity` is a separate, narrower type and the cards/list converter hand-rolls it without going through `serializeRelatedEntityForWire` or `stripHiddenProperties`. Today the upstream `sec.Fields` view config masks the gap — no per-row ACL hidden evaluation runs on this path).

   The actual predicate is `App.hiddenProperties(ctx, e) map[string]struct{}` at `affordances.go:694`. The new helper `copyVisibleProperties(ctx, e)` calls it and copies only properties whose name is NOT in the hidden set:
   ```go
   func (a *App) copyVisibleProperties(ctx context.Context, e *entity.Entity) map[string]any {
       hidden := a.hiddenProperties(ctx, e)
       out := make(map[string]any, len(e.Properties))
       for k, v := range e.Properties {
           if _, h := hidden[k]; h { continue }
           out[k] = v  // shallow copy is sufficient (RR-FD1E #6)
       }
       return out
   }
   ```
   `computeFieldAffordances` already drops hidden keys (affordances.go:621-622, 632) so `_fields` is hidden-stripped without further intervention.

6. **Frontend `ViewEntity` type extended** in `frontend/src/api/views.ts`:
   ```ts
   export interface ViewEntity {
     // ... existing fields
     _props?: Record<string, unknown>
     _fields?: Record<string, FieldAffordance>
   }
   ```
   No consumer wires these up in this ticket — TKT-IHC7C does that.

7. **Wire-format docs updated** in `docs/data-entry/api-reference.md` — one paragraph under the View section explaining the new fields.

8. **Backend tests:**
   - Cards/list section response includes `_props` matching `e.Properties` (filtered through `hiddenProperties`) for the seeded fixture entities.
   - Cards/list section response includes `_fields` with the same shape `computeFieldAffordances` returns for a per-entity GET of the same entity (consistency).
   - Hidden properties are absent from `_props` (RR-FD1A — new test pinning the new behaviour).
   - Key-set invariant (RR-FD1B): `keys(_props) ⊆ keys(e.Properties) \ hidden(e)` AND `keys(_fields) ∩ hidden(e) == ∅`.
   - `_fields` semantics match V1Entity: absent if not computed, present-but-empty `{}` if evaluated with no deviations.
   - `_props` is NOT the same map pointer as `e.Properties` — defensive `reflect.ValueOf(...).Pointer()` check on a freshly built response (RR-FD1E #7 — replaces the round-1 mutate-and-refetch test which would have passed trivially via JSON marshaling).
   - Both `properties`/`list` AND `content`/`cards` branches produce identical wire shapes (RR-FD1C — both paths covered).

9. **Frontend regression:** `npm run typecheck` clean; existing wire-decoding tests unchanged.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: surgical wire extension)
- [x] Searched for existing libraries — N/A
- [x] Checked codebase for similar patterns — yes (V1Entity.FieldAffordances precedent)
- [x] Looked for reference implementations — N/A
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:**

- `V1Entity.FieldAffordances *map[string]V1FieldAffordance` at `api_v1.go` — established pointer-to-map idiom for "absent vs empty" distinction. Mirror this.
- `App.computeFieldAffordances(ctx, e)` at `affordances.go:612` — already returns the sparse per-entity affordance map. Reusable for row entities.
- `App.attachEntityAffordances` at `affordances.go:786` — attaches `_fields` and `_relations` to a per-entity response. For rows we want only `_fields`, not `_relations` (per ticket non-goals), so we call `computeFieldAffordances` directly rather than the full attach.
- `App.hiddenProperties(ctx, e) map[string]struct{}` at `affordances.go:694` — the predicate for per-row hidden-property filtering. Reused by the new `copyVisibleProperties` helper (RR-FD1A).
- `serializeRelatedEntityForWire` at `affordances.go:806` — operates on `V1Entity`. Its comment ("omits the `_fields` / `_relations` maps") is correct for `V1Entity` and stays unchanged (RR-FD1D). `V1ViewEntity` is a separate, narrower type; this ticket adds inline-edit affordances to that type without touching the `V1Entity` contract.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach** (rewritten in round 2 to match the AC section; see RR-FD2A):

1. **Add `Props` and `Fields` to `SectionEntityData`** in `sections.go`:
   ```go
   type SectionEntityData struct {
     // ... existing fields
     Props  map[string]any                 // typed properties from e.Properties, hidden-stripped
     Fields map[string]V1FieldAffordance   // sparse per-cell affordance verdict
   }
   ```
   No `entity` back-reference (RR-FD1E). The wire converter consumes the precomputed maps directly.

2. **Add the `copyVisibleProperties` helper** in (likely) `affordances.go`, alongside the existing `hiddenProperties` predicate:
   ```go
   func (a *App) copyVisibleProperties(ctx context.Context, e *entity.Entity) map[string]any {
       hidden := a.hiddenProperties(ctx, e)
       out := make(map[string]any, len(e.Properties))
       for k, v := range e.Properties {
           if _, h := hidden[k]; h { continue }
           out[k] = v  // shallow copy is sufficient — JSON marshaling handles deep
       }
       return out
   }
   ```

3. **Extract the shared `buildSectionEntityData` helper** in `sections.go` and call it from BOTH non-entry branches (RR-FD1C — `properties`/`list` at L192-219 AND `content`/`cards` at L279-308):
   ```go
   func (a *App) buildSectionEntityData(ctx context.Context, e *entity.Entity, secFields []ViewField, eDef *metamodel.EntityDef) SectionEntityData {
       sed := SectionEntityData{
           ID:         e.ID,
           Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
           Type:       e.Type,
           EditFormID: a.editFormForType(e.Type),
           Props:      a.copyVisibleProperties(ctx, e),
           Fields:     a.computeFieldAffordances(ctx, e),
       }
       // existing per-section-field string-building loop:
       for _, f := range secFields { /* sed.Fields append (display string fields) */ }
       return sed
   }
   ```
   Note: the existing `SectionFieldData` (string-fields per section config) is a separate concept from the new `Fields map` (per-cell affordance verdict). To avoid name collision, rename the new map to `FieldVerdicts` or similar during implementation; AC 4 wire-converter snippet follows whatever name lands.

4. **Wire converter snippet at ALL `V1ViewEntity` construction sites** in `api_v1.go` (RR-FD2A — both `~L3010` for top-level entities AND `L3066-3077` for group-card display):
   ```go
   v1Ent.Props = e.Props
   if e.Fields != nil {
       fields := e.Fields
       v1Ent.FieldAffordances = &fields
   }
   ```

5. **Update TS types in `frontend/src/api/views.ts`** — extend `ViewEntity` with `_props?: Record<string, unknown>` and `_fields?: Record<string, FieldAffordance>`. No further frontend work; TKT-IHC7C wires consumption.

6. **Update docs** in `docs/data-entry/api-reference.md` — one paragraph under the View response section explaining the new fields and their semantics.

**Files to modify:**

- `internal/dataentry/api_v1.go` — `V1ViewEntity` struct + wire conversion at the cards/list path.
- `internal/dataentry/sections.go` — `SectionEntityData` struct + `buildSections` populates `Props` and `entity` for cards/list rows.
- `internal/dataentry/sections_test.go` (or wherever the view tests live) — new test cases for `_props` and `_fields` on cards/list responses.
- `frontend/src/api/views.ts` — extend `ViewEntity` with `_props` and `_fields`.
- `docs/data-entry/api-reference.md` — paragraph on the new wire fields.

**Alternatives considered:**

- *(a) Re-lookup the entity from the store at wire-conversion time, keyed by `SectionEntityData.ID`.* Rejected — same data, extra store lookup, defeats the section-builder's batching.
- *(b) Compute `_fields` inside `buildSections` and store on `SectionEntityData`.* **ADOPTED** (RR-FD1E — round 1 rejected this on the wrong reasoning; `buildSections` already takes `ctx` from its handler). Storing precomputed `Props` and `Fields` on `SectionEntityData` keeps the wire converter dumb and the package boundaries clean.
- *(c) Skip `_fields` for now; only ship `_props`.* Rejected — TKT-IHC7C needs both; splitting them defers the integration without saving complexity.
- *(d) Thread `*entity.Entity` through to the wire converter via an unexported back-reference.* Rejected via (b) ADOPTED — the wire converter doesn't need the entity if `_fields` is precomputed.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `e.Properties` — already validated by storage layer; trusted at this point.
- Hidden-property predicate — same predicate as the entry-level path, no new evaluation logic.

**Security-Sensitive Operations:**

- Hidden-property leakage. `_props` MUST filter through the hidden-property predicate so encrypted / restricted fields don't appear in the wire map. Tested explicitly.
- Per-row affordance computation runs the ACL resolver once per row. For large lists this could cost — but the same path already runs for the entity-level GET, and cards/list typically have <50 rows. Not a new attack surface.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

- **AC 1+4** (wire shape on cards/list): GET a view with a cards section; assert each row carries `_props` matching the entity's properties and `_fields` matching the per-entity verdict.
- **AC 3** (defensive copy): mutate the returned `_props` map; ensure the next GET still returns the unmutated baseline.
- **AC 5** (hidden-property stripping): seed an entity with a property marked hidden in the metamodel; assert it's absent from `_props`.
- **AC 6** (TS types): `npm run typecheck` passes after the type extension.
- **AC 8** (`_fields` consistency): same entity surfaced via per-entity GET and via cards/list row produces identical `_fields` maps.

**Edge Cases:**

- Cards section with zero entities → `_props` and `_fields` simply don't appear.
- Entity with no readable properties (all encrypted) → `_props: {}`, `_fields` reflects each property as `writable: false` or hidden out entirely (depending on metamodel + ACL).
- Per-entity GET and cards row inconsistency would indicate a bug in `computeFieldAffordances` reuse. Test pins the consistency.

**Negative Tests:**

- Table sections (`display: 'table'`) → `_props` and `_fields` MUST NOT appear (out of scope for this ticket, table cells handle stringified values differently).
- Entry section (`source: 'entry'`) → `_props` is not populated; the existing `V1Entity.Properties` shape is the entry's source of truth.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated — `s`

**Risks:**

- **ACL re-eval cost on large lists.** Mitigation: only fires for cards/list display, not table. For very-large cards lists this could add 20-100ms; not blocking. If it becomes a problem, batch the ACL resolver call per-list (out of scope for this ticket).
- **Hidden-property predicate drift.** Mitigation: reuse the existing predicate, don't reimplement. Test pins the rule.
- **TS-type vs Go-type drift.** Mitigation: AC 9 typecheck + the new backend tests assert the shape; manual cross-check during implementation.

## Documentation Planning

- [x] User-facing docs identified — `docs/data-entry/api-reference.md` (AC 7)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: one paragraph in api-reference covers it; no guide/tutorial impact)

**Documentation Impact:**

- [x] `docs/data-entry/api-reference.md` — new wire fields under View response section.

## Design Review

- [x] Run `/design-review` before starting implementation — rounds 1 & 2 complete; 6 findings rolling up 12 concerns captured as RR-FD1A..RR-FD1E + RR-FD2A
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| Finding | Severity | Status | Disposition |
|---|---|---|---|
| RR-FD1A | critical | addressed | Reframe Scope #3: introduce hidden-stripping (not "already done"); use `hiddenProperties` predicate (not `IsHiddenForType`) |
| RR-FD1B | significant | addressed | Key-set invariant pinned in Go doc + new backend test |
| RR-FD1C | significant | addressed | Both `properties`/`list` AND `content`/`cards` branches wired via shared helper |
| RR-FD1D | significant | addressed | `serializeRelatedEntityForWire` comment left intact; new V1ViewEntity doc explains its own contract |
| RR-FD1E | minor | addressed | Pointer-to-map only on `_fields`; reverse alt-b rejection (compute in buildSections, drop entity back-reference); replace AC 3 mutate test with map-pointer check |
| RR-FD2A | minor (round 2) | addressed | Third converter site for `GroupData.Entities` covered in AC 4; stale Technical Approach section rewritten to match ACs |
