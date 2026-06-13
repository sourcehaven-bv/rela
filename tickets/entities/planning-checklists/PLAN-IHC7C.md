<!-- @managed: claude-workflow v1 -->
---
id: PLAN-IHC7C
type: planning-checklist
title: 'Planning: Cards/list inline edit'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** see TKT-IHC7C ticket body. Frontend-only; TKT-IHC7B (SectionEditForm) + TKT-IHC7D (`_props` + `_fields` per row) shipped the prerequisites. Wraps `SectionEditForm` around each row in EntityDetail's cards and list sections; per-cell writability via the row's `_fields`; one AutoSaveIndicator per row.

**Acceptance Criteria:**

1. **Cards section gets inline-edit per-row.** EntityDetail's `section.display === 'cards'` branch wraps each row's field list in `<SectionEditForm>` when `sectionShouldRouteToInlineEdit` returns true for that row (mirroring IHC7B's decision for the entry's properties section). Non-editable rows — including rows where any field is inaccessible (RR-FC1E NEW-4) — continue to render via the existing display-mode `fieldRowsFor` path. Mixed sections (some rows editable, some not) render correctly.

2. **List section gets inline-edit per-row.** Same treatment as cards. The visual chrome stays as the existing `<ul>/<li>` list; the per-row autosave indicator sits inline (e.g. right of the row's title).

3. **Each row's `SectionEditForm` is keyed on `${ent.type}/${ent.id}`** so route-style remount semantics work for in-row navigation (consistent with IHC7B's pattern on the entry's properties section).

4. **Each row's `initialValues` come from `ent._props` (typed).** When a row's `_props` is absent (legacy server), fall back to PropertyDisplay rendering — no inline-edit. This is a defensive fallback; the post-IHC7D server always sends `_props` for cards/list rows.

5. **Per-cell writability** uses the row's `_fields` verdict via the existing `buildSectionEditFields` + `sectionShouldRouteToInlineEdit` helpers, **parameterized over a loosened `FieldVerdictSource = { type: string; _fields?: Record<string, FieldAffordance> }` shape** (RR-FC1A). Both `Entity` (entry section) and `ViewEntity` (row) satisfy the shape — no new helpers required. `applyPropertyToRow` is the only genuinely new helper because `Entity.properties` and `ViewEntity._props` are different storage shapes.

   **String mirror sync (RR-FC1C):** the row's display-mode cells (`fieldRowsFor` consumers in cards + list templates) prefer `ent._props?.[f.property]` over `f.values` when `_props` is present. This eliminates the stale-mirror class of bugs when a row's writable verdict flips back to display mode mid-session. `f.values` (display-stringified) is the legacy fallback for rows that have no `_props` on the wire.

6. **One AutoSaveIndicator per row, host-controlled placement via slot + Teleport (RR-FC1D + RR-FC2A).** SectionEditForm gains a scoped named slot exposing `{ status, error }`:
   ```vue
   <slot name="indicator" :status="autoSave.status.value" :error="autoSave.lastError.value">
     <AutoSaveIndicator :status="autoSave.status.value" :error="autoSave.lastError.value" />
   </slot>
   ```
   Cards/list call sites use Vue `<Teleport>` to place the AutoSaveIndicator into a marker `<span class="card-indicator-slot">` or `<span class="list-indicator-slot">` rendered in the host chrome. No `rowStatus`/`rowError` accessor pattern. The slot's default preserves IHC7B current behaviour for the entry-section caller.

7. **Owner-identity guard on writeback (RR-FB2A + RR-FC1E NEW-3).** Each row's `onPropertyApplied(prop, value, owner)` is dispatched via a memoized `Map<\`${type}/${id}\`, { sectionIdx, rowIdx }>` (rebuilt per `viewData` assignment) — O(1) lookup instead of O(sections × rows). The host updates the matching row via `applyPropertyToRow(ent, prop, value, owner)`, which spread-clones `_props`. The section's `entities` array is spread-cloned as `{ ...section, entities: section.entities.map((e, i) => i === targetIdx ? nextEnt : e) }` — preserves other rows' references so the WeakMap-keyed memo cache stays valid for them. Identity-guard rejects stale responses (the row's id no longer at that sectionIdx/rowIdx — either deleted or repositioned). Tested under sort/group reorder.

8. **Editing a row does NOT navigate to the row's entity (RR-FC1B).** Today clicking a card or list item navigates to `entity/<type>/<id>` because `@click="navigateToEntity(ent)"` is on the row-level `<article>`/`<li>`. Move the handler to a more specific subelement: `.card-header` (cards) and `.list-link` (list). SectionEditForm stays uncoupled from its host's navigation semantics — no `@click.stop` inside the form. Test asserts: click on a widget input does NOT navigate; click on the title does.

9. **Existing display-mode tests pass unchanged.** Cards/list display mode for non-writable sections renders identically; e2e suites that exercise list/cards navigation continue to work.

10. **New unit tests:**
    - `buildSectionEditFields` (parameterized): assert correct discriminated-union output when fed an `Entity` (entry section, existing case) AND a `ViewEntity` (row, new case). One helper, two fixture shapes (RR-FC1A).
    - `sectionShouldRouteToInlineEdit` (parameterized): same — Entity AND ViewEntity fixtures.
    - `applyPropertyToRow`: stale-owner rejection; correct spread-clone shape with other rows' references preserved (RR-FC1E NEW-3).
    - EntityDetail integration: mount with a cards section where one row has `_fields: { status: { writable: false } }` and another has `_fields: {}`; assert the first row's `status` widget is `mode='display'` and the second's is editable.
    - Owner-identity guard under reorder: simulate a row's PATCH resolving after the section reorders so the target row is at a different index; assert the writeback finds the matching row by `(type, id)` and not by index.
    - Click propagation: assert clicking a widget input does NOT trigger `navigateToEntity`; clicking the title link DOES (RR-FC1B).
    - Smoke: 100-row cards section mounts under 200ms wall-clock (RR-FC1D).
    - Cap behaviour: 101-row cards section produces ZERO SectionEditForm instances (display-mode fallback above the cap) — `wrapper.findAllComponents(SectionEditForm).length === 0` (RR-FC2C).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: pattern established in IHC7B; this ticket applies it per-row)
- [x] Searched for existing libraries — N/A
- [x] Checked codebase for similar patterns — `fieldRowsFor`, `buildSectionEditFields`, `applyPropertyToEntry`
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:**

- `SectionEditForm.vue` (IHC7B) — the per-section autosave host. Already accepts `entityType`, `entityId`, `initialValues`, `fields`, `onPropertyApplied`, `onError`, `onVerdictFlip` props. Reusable per-row without changes.
- `sectionEditFields.ts` (IHC7B) — pure helpers for building the discriminated-union `fields` prop and applying confirmed writes. `buildSectionEditFields` currently takes the *entry's* fields and `_fields` map; we add a per-row caller variant.
- `applyPropertyToEntry` (IHC7B) — applies a confirmed property write to a target Entity using owner-identity guard. The row equivalent applies to a `ViewEntity` (different shape: `_props` is the typed map, no `properties` field).
- `fieldRowsFor` (UD7YR, EntityDetail.vue:450) — current per-row widget resolution for cards/list display mode. Stays for non-editable rows.
- TKT-IHC7D `V1ViewEntity._props` + `._fields` (shipped) — the typed row data this ticket consumes.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach** (revised in round 1 to reflect RR-FC1A..RR-FC1E):

1. **Loosen `sectionEditFields.ts` helper signatures** (RR-FC1A):
   - `buildSectionEditFields(fields, source, getPropertyDef)` where `source: FieldVerdictSource = { type: string; _fields?: Record<string, FieldAffordance> }`. Both `Entity` and `ViewEntity` satisfy this.
   - `sectionShouldRouteToInlineEdit(section, source, getPropertyDef)` — same parameter loosening.
   - No new `buildRowEditFields` / `rowShouldRouteToInlineEdit` — the existing helpers do double duty.

2. **Add `applyPropertyToRow(ent: ViewEntity, prop: string, value: unknown, owner) → ViewEntity | null`** to `sectionEditFields.ts`. Owner-identity guard against `(ent.type, ent.id)`; spread-clones `_props`. Does NOT touch `fields[i].values` — the row's display mode reads from `_props` first (RR-FC1C), so the string mirror is no longer load-bearing for verdict-flip cases.

3. **Add `SectionEditForm.vue` named slot for the indicator** (RR-FC1D + RR-FC2A):
   ```vue
   <template>
     <div class="section-edit-form">
       <slot name="indicator" :status="autoSave.status.value" :error="autoSave.lastError.value">
         <AutoSaveIndicator :status="autoSave.status.value" :error="autoSave.lastError.value" />
       </slot>
       <!-- ...existing template... -->
     </div>
   </template>
   ```
   The slot exposes `status` and `error` as scope props so the host can render the indicator anywhere (e.g. teleported into the card header). Default preserves IHC7B current behaviour. Existing entry-section call site requires no change.

4. **EntityDetail click-handler moves** (RR-FC1B):
   - Cards: move `@click="navigateToEntity(ent)"` from `<article>` to `.card-header`.
   - List: move from `<li>` to `.list-link`.
   - SectionEditForm stays uncoupled; no `@click.stop` inside the form.

5. **EntityDetail cards branch integration** (RR-FC2A — indicator placement is host-controlled via the slot AND Vue `<Teleport>`; no escape hatch):
   ```vue
   <article v-for="ent in section.entities" :key="ent.id" class="entity-card">
     <header class="card-header" @click="navigateToEntity(ent)"> <!-- moved from article -->
       <span class="entity-type">{{ ent.type }}</span>
       <span class="entity-title">{{ ent.title }}</span>
       <span class="entity-id">{{ ent.id }}</span>
       <!-- Marker for the SectionEditForm to teleport its indicator into.
            Empty when the row is not inline-editable. -->
       <span :id="`card-indicator-${ent.id}`" class="card-indicator-slot"/>
       <button v-if="ent.editFormId" class="edit-btn" @click.stop="navigateToEdit(ent.editFormId, ent.id)">×</button>
     </header>
     <SectionEditForm
       v-if="sectionShouldRouteToInlineEdit(ent, ent, getPropertyDef)"
       :key="`${ent.type}/${ent.id}`"
       :entity-type="ent.type"
       :entity-id="ent.id"
       :initial-values="ent._props ?? {}"
       :fields="memoBuildRowEditFields(section, ent)"
       :on-property-applied="handleRowPropertyApplied"
       :on-error="handleSectionEditError"
       :on-verdict-flip="handleVerdictFlip"
     >
       <!-- Teleport the indicator into the card-header slot above.
            Keeps SectionEditForm uncoupled from card layout; host owns placement. -->
       <template #indicator="{ status, error }">
         <Teleport :to="`#card-indicator-${ent.id}`">
           <AutoSaveIndicator :status="status" :error="error" />
         </Teleport>
       </template>
     </SectionEditForm>
     <div v-else-if="ent.fields?.length" class="card-fields">
       <!-- existing fieldRowsFor display-mode rendering; prefer ent._props over field.values -->
     </div>
   </article>
   ```
   The named slot exposes `{ status, error }` as scope props (a small extension to SectionEditForm's slot — additive, no breaking change for the entry-section caller which uses the default).

6. **EntityDetail list branch integration:** Same pattern, click-handler on `.list-link`, slot override puts indicator inline-right of title.

7. **`handleRowPropertyApplied(prop, value, owner)`** (RR-FC1E NEW-3):
   - Look up `(sectionIdx, rowIdx)` in a memoized `rowIndex: Map<string, { sectionIdx, rowIdx }>` rebuilt per viewData assignment (a single watch on `viewData.value`).
   - If not found: bail.
   - Apply via `applyPropertyToRow(currentEnt, prop, value, owner)`.
   - Spread-clone: `viewData.value = { ...vd, sections: vd.sections.map((s, i) => i === sectionIdx ? { ...s, entities: s.entities.map((e, j) => j === rowIdx ? nextEnt : e) } : s) }`.

8. **`memoBuildRowEditFields(section, ent)`**: WeakMap keyed on `ent` reference (parallel to IHC7B's `sectionEditFieldsCache`). Returns the discriminated-union field list. Invalidated naturally when `applyPropertyToRow` produces a new `ent` reference.

9. **Display-mode read from `_props` first** (RR-FC1C): the existing `fieldRowsFor` template path uses `:model-value="row.field.values ?? []"`. Update to `:model-value="ent._props?.[row.field.property] ?? row.field.values ?? []"`. One small template change, eliminates the stale-mirror class of bugs.

10. **Soft cap: 100 rows per inline-edit section** (RR-FC1D). When `section.entities.length > 100`, all rows render in display mode. Above this threshold the user is more likely browsing than editing. Configurable later if needed.

11. **Grouped cards: not wired** (RR-FC1C / S2). Backend doesn't produce `Groups.entities` for cards today. Code comment in the cards branch: "Grouped cards have no backend producer today; when added, this path needs parallel wiring."

**Files to modify:**

- `frontend/src/components/entity/sectionEditFields.ts` — add `rowShouldRouteToInlineEdit`, `buildRowEditFields`, `applyPropertyToRow`.
- `frontend/src/components/entity/sectionEditFields.test.ts` — extend tests for the row variants.
- `frontend/src/components/entity/EntityDetail.vue` — wire SectionEditForm into cards + list branches; add `handleRowPropertyApplied` and the memoization helper; click-propagation handling.
- `frontend/src/components/forms/SectionEditForm.vue` — small CSS tweaks for the indicator's placement in card/list contexts (no API change).
- Tests for the integration (likely a new harness similar to SectionEditForm's mount-tests, scoped to the row variant).

**Alternatives considered:**

- *(a) Inline-edit at the cell level — each cell its own autosave.* Rejected — N×M autosave instances for an M-field row; per-cell debounces would race; AutoSaveIndicator placement becomes unclear.
- *(b) One section-level SectionEditForm for the entire cards section, addressing rows via index.* Rejected — SectionEditForm's contract is "one entity, one form." Per-row identity is the natural grain; the host already knows per-row identity from `ent.id`.
- *(c) Defer to a follow-up: only support editing on cards, not list.* Rejected — both paths share the same iteration shape; doing them together avoids two PRs of overlapping refactor.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Row `_props` and `_fields` — server-supplied, hidden-property-stripped at the source by TKT-IHC7D. Trusted at consumption.
- PATCH submissions — go through existing `entitiesStore.update` path; server re-authorizes per row.

**Security-Sensitive Operations:**

- Row PATCH submission targets a different entity than the page's entry. The server's ACL must authorize writes on the row entity, not the entry — confirmed by re-reading the PATCH path in the data-entry server (TKT-IHC7B's structured `onError` extension already passes the row entity via the SectionEditForm props). 401/403 on a row triggers `loadView()` once, same as the entry's section.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

- AC 1+2 (cards/list inline edit): mount EntityDetail with a fixture cards section + a fixture list section; both have rows with writable fields. Assert the SectionEditForm renders per row.
- AC 3 (`:key` per row): assert each `SectionEditForm` has the expected `:key`.
- AC 4 (legacy fallback): mount with a row missing `_props`; assert that row uses display-mode (PropertyDisplay or fieldRowsFor).
- AC 5 (per-cell writability): mount with one row's `_fields` denying `status`; assert that cell renders `mode='display'`.
- AC 6 (indicator per row): assert each editable row has its own `<AutoSaveIndicator>` (count match).
- AC 7 (owner-identity guard): unit test on `applyPropertyToRow` with a stale row identity; assert null.
- AC 8 (click propagation): simulate click on a widget inside a row; assert `navigateToEntity` NOT called.
- AC 9 (regression): existing e2e cards/list tests pass unchanged.
- AC 10 (helper unit tests): full coverage on `buildRowEditFields` and `rowShouldRouteToInlineEdit`.

**Edge Cases:**

- A row's `_props: {}` (no properties): row's `rowShouldRouteToInlineEdit` returns false → display mode.
- A row's `_fields: {}` (no writable fields): same — no inline-edit needed.
- A row's `_props` and `_fields` both present but no field in the row has a writable verdict: display mode.
- 50+ rows: 50 SectionEditForm instances. Each has its own timer + queueTail; idle cost is small. Worth a performance smoke test (not strict perf gate).
- A row's entity is deleted server-side between PATCH and response: `handleRowPropertyApplied` finds no matching row → bail.
- Cards section with `isGrouped: true`: groups have their own `.entities` arrays (parallel to `.rows` for table). For cards, `isGrouped` is rare in practice (today's data only groups table sections), but if it ships we apply the same SectionEditForm wrapping inside each group's loop.

**Negative Tests:**

- Click on a widget input field inside a row: `navigateToEntity` NOT called.
- Row PATCH 403: `handleSectionEditError` triggers `loadView()` once.
- Row verdict-flip mid-session: SectionEditForm's existing `onVerdictFlip` watcher fires.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated — `m` (effort dropped from `l` since IHC7B + IHC7D shipped the heavy lifting; this is reuse of established patterns)

**Risks:**

- **Click propagation on cards.** The card-level `@click="navigateToEntity(ent)"` listens on the `<article>`. Need to either `@click.stop` on the SectionEditForm cells (or, cleaner, move the navigate handler to a specific subelement like the card title). The cleaner refactor breaks no existing tests because the click target is still inside the card; do that.
- **Per-row instance count.** 50 SectionEditForm instances costs ~50 useAutoSave closures and timers. Each is light; idle cost is fine. Worth one smoke test that a section with 100 rows mounts without slowness.
- **Memoization to keep verdict watchers stable.** Each row's `fields` prop must be memoized per (section, row) so the IHC7B verdict-flip watcher doesn't fire on every reactive tick. Use a WeakMap keyed on the ViewEntity reference, mirroring the existing `sectionEditFieldsCache`.
- **List item interactive height growth.** Today a list item is single-line; with editable widgets, the item becomes multi-line. CSS-only; document the visual delta but don't try to constrain it.

## Documentation Planning

- [x] User-facing docs identified — N/A
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: internal Vue SFC + composable wiring; the user-visible behaviour change — inline-edit on cards/list rows — is self-evident in the UI and consistent with the entry's properties section already shipped by IHC7B)

**Documentation Impact:** N/A — internal change building on shipped surfaces.

## Design Review

- [x] Run `/design-review` before starting implementation — rounds 1 & 2 complete; 8 findings total
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| Finding | Severity | Status | Disposition |
|---|---|---|---|
| RR-FC1A | critical | addressed | Parameterize existing helpers; no new buildRowEditFields/rowShouldRouteToInlineEdit |
| RR-FC1B | significant | addressed | Move navigateToEntity to .card-header / .list-link; no @click.stop in form |
| RR-FC1C | significant | addressed | Display reads _props first; grouped-cards branch dropped (no backend producer) |
| RR-FC1D | significant | addressed | Slot for indicator placement; 100-row soft cap with smoke test |
| RR-FC1E | minor | addressed | Memo Map for row index; clone shape pinned; defensive comments documented |
| RR-FC2A | significant | addressed | Step 5 example committed to slot + Teleport; no rowStatus/rowError escape hatch |
| RR-FC2B | critical | addressed | False positive — IHC7D shipped after branch creation; rebase brings it in |
| RR-FC2C | minor | addressed | Cap-behaviour test separated from perf smoke |
