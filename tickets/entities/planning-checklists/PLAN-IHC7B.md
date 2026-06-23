---
id: PLAN-IHC7B
type: planning-checklist
title: 'Planning: Properties-section inline edit via SectionEditForm'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** see TKT-IHC7B ticket body. Builds on TKT-IHC7A (per-channel debounce,
`initialServerSnapshot`, channel disable on `useAutoSave`) and TKT-UD7YR
(view-side widget delegation, `WidgetRoutingHint`, `mode` required on widgets).
**Reframed per design-review round 1**: the `WidgetMode` widening is dropped
(RR-FB1F); SelectWidget's transitions panel is suppressed by not passing
`transitions`, not by introducing a third render mode.

**Implementation gate:** Confirm PR #912 (TKT-IHC7A) has merged into develop
before starting implementation (RR-FB1O / N4). Re-merge develop into this branch
and re-run the typecheck + tests.

**Acceptance Criteria:**

1. **`useAutoSave.onError` signature extended (RR-FB1K).** `AutoSaveOptions.onError: (msg: string, info?: { status?: number; property?: string; channel?: 'property' | 'content' | 'relations' }) => void`. Additive change; existing callers (DynamicForm, EntityDetail's contentAutoSave) ignore `info`. Test: scheduleFieldSave with mocked 403 response — onError invoked with `info.status === 403, info.property === <name>, info.channel === 'property'`.

2. **Shared affordance helper extracted (RR-FB1I + RR-FB2B).** A new `frontend/src/utils/affordances.ts` exports:
   ```ts
   export function isFieldWritable(verdict: FieldAffordance | undefined, fieldReadonly?: boolean): boolean
   export function optionVerdictsFor(verdict: FieldAffordance | undefined): Record<string, boolean> | undefined
   ```
`isFieldWritable` returns `!fieldReadonly && verdict?.writable !== false` —
preserves DynamicForm's static-readonly channel (RR-FB2B). DynamicForm's
existing inline helpers (DynamicForm.vue L185-199) are refactored to call these,
passing `field.readonly`. SectionEditForm passes `undefined` for `fieldReadonly`
(no static-readonly concept on this surface). Tests cover both channels
independently AND combined.

3. **Shared cleared-value helper extracted (RR-FB1E).** Move DynamicForm's `isClearedForType(value, def)` into `frontend/src/utils/formValue.ts`. SectionEditForm imports it and routes `update:modelValue` through:
   ```ts
   if (isClearedForType(value, propertyDef)) autoSave.scheduleUnset(property)
   else autoSave.scheduleFieldSave(property, value)
   ```
Tests: text cleared → scheduleUnset; multi-select cleared to `[]` →
scheduleUnset; non-empty → scheduleFieldSave.

4. **`SectionEditForm.vue` component (RR-FB1H discriminated union, RR-FB2A owner identity)** under `frontend/src/components/forms/`. Props:
   ```ts
   defineProps<{
     entityType: string
     entityId: string
     initialValues: Record<string, unknown>
     fields: SectionEditField[]
     // Owner identity (entityType + entityId) is captured here from props at mount and forwarded to
     // every callback so the host can reject stale responses from a previous-entity instance whose
     // PATCH was flushed during :key-driven unmount (RR-FB2A).
     onPropertyApplied: (prop: string, value: unknown, owner: { type: string; id: string }) => void
     onError: (msg: string, info?: { status?: number; property?: string; channel?: 'property' | 'content' | 'relations' }) => void
     onVerdictFlip?: (prop: string, label: string) => void
   }>()

   type SectionEditField = {
     property: string  // required
     label: string
     verdict?: FieldAffordance
   } & (
     | { kind: 'schema'; propertyDef: PropertyDef }
     | { kind: 'hint';   routingHint: WidgetRoutingHint }
   )
   ```

Implementation:
   - Owner identity captured at construction: `const owner = { type: props.entityType, id: props.entityId }` (frozen for the instance's lifetime — when route changes, the `:key` remount creates a new instance with new identity).
   - `useAutoSave` with `disableContentChannel: true`, `disableRelationsChannel: true`, `initialServerSnapshot: { properties: { ...initialValues } } as Entity` (NEW-10: spread independent of formData), and the following callbacks:
     ```ts
     applyServerProperty: (prop, value) => {
       if (value === undefined) delete (formData as any)[prop]   // RR-FB2D NEW-5: parity with DynamicForm L923-929
       else (formData as any)[prop] = value
       try { onPropertyApplied(prop, value, owner) }
       catch (e) { console.error(e) /* RR-UE3D: don't rollback */ }
     },
     onError: (msg, info) => onError(msg, info),
     ```
   - The remaining required AutoSaveOptions fields are passed as no-ops (RR-FB2D NEW-9):
     ```ts
     contentRef: ref(''),
     inverseToCanonical: new Map(),
     buildRelationsBody: () => null,
     applyServerContent: () => {},
     formData: computed(() => formData) as unknown as Ref<Record<string, unknown>>,
     ```
   - Widget resolution: `field.kind === 'schema' ? defaultRegistry.resolve(undefined, field.propertyDef) : defaultRegistry.resolveFromHint(field.routingHint)`. Precomputed via `computed(() => fields.map(...))` mirroring PropertyDisplay L42-64.
   - Per-cell render: writable cells use `<FieldShell :error="autoSave.fieldErrors[field.property]">` wrapping the widget in `mode='edit'` with `v-model="formData[field.property]"`; non-writable cells render the widget in `mode='display'`, no FieldShell, no v-model. NO `transitions` prop is ever passed (RR-FB1F suppresses SelectWidget's hint panel).
   - One `AutoSaveIndicator` rendered inline at the section heading (RR-UE3H; layout via the host).
   - `onBeforeUnmount → commitImmediately()`.
   - `watch(() => props.fields, (next, prev) => { ... })` detects per-property writable flips `true → false`. For each such property: `autoSave.revertField(prop)` (already exists) + `onVerdictFlip?.(prop, field.label)` (RR-FB2C: separate from `onError` to avoid 403-refetch loop). Verdict flips do NOT trigger loadView again — the loadView that surfaced the new verdict is what raised them.

5. **EntityDetail integration (RR-FB1D :key remount + RR-FB1J property filter + RR-FB1G spread-clone writeback + RR-FB2A owner identity guard).**
   - Helper `buildSectionEditFields(section, entry)`: filter `section.fields` to `f.property !== undefined`; for each: resolve `verdict = entry._fields?.[f.property]`; build the discriminated-union shape (`kind: 'schema'` when schema def found, else `kind: 'hint'`).
   - Memoize via `computed` keyed off the section + entry reference so the resulting array's identity stabilises across reactive ticks (RR-FB2D NEW-4); the SectionEditForm watch only fires when verdicts actually change.
   - Helper `sectionHasAnyWritable(section, entry)`: `buildSectionEditFields(...).some(f => isFieldWritable(f.verdict))`.
   - Template:
     ```vue
     <SectionEditForm
       v-if="section.display === 'properties' && sectionHasAnyWritable(section, entry)"
       :key="`${entry.type}/${entry.id}`"
       :entity-type="entry.type"
       :entity-id="entry.id"
       :initial-values="entry.properties"
       :fields="buildSectionEditFields(section, entry)"
       :on-property-applied="handlePropertyApplied"
       :on-error="handleSectionEditError"
       :on-verdict-flip="handleVerdictFlip"
     />
     <PropertyDisplay v-else-if="section.display === 'properties'" ...current... />
     ```
   - `handlePropertyApplied(prop, value, owner)` (RR-FB2A identity guard + RR-FB2D NEW-5 undefined-as-delete):
     ```ts
     const view = viewData.value
     if (!view?.entry) return
     if (view.entry.type !== owner.type || view.entry.id !== owner.id) return  // stale response from previous-entity SectionEditForm instance
     const nextProps = { ...view.entry.properties }
     if (value === undefined) delete nextProps[prop]
     else nextProps[prop] = value
     viewData.value = { ...view, entry: { ...view.entry, properties: nextProps } }
     ```
   - `handleSectionEditError(msg, info)`: if `info?.status === 401 || info?.status === 403` (RR-FB2D NEW-6), call `loadView()` ONCE (de-dupe via a `pendingRefetch` flag cleared on response). Always call `uiStore.error(msg)`.
   - `handleVerdictFlip(prop, label)`: `uiStore.warning(\`Permission changed — your unsaved edit to '${label}' was discarded\`)`(RR-FB2C: NOT routed through`handleSectionEditError`; no loadView refetch — the loadView that surfaced the verdict is what triggered this notification).
   - The `:key="entry.type/entry.id"` forces SectionEditForm remount on route navigation between entities. On remount: previous instance's `onBeforeUnmount → commitImmediately` flushes the pending PATCH while `getEntityType/getEntityId` still close over the previous props. The owner-identity guard in `handlePropertyApplied` then ensures the asynchronously-arriving response does NOT splice into the new entity's view (RR-FB2A).

6. **Verdict-flip semantics (RR-FB1M + RR-FB2C / RR-UE3G).** When `entry._fields[prop].writable` flips `true → false` between two `loadView()` cycles: SectionEditForm's `watch` on `fields` prop detects the transition, calls `revertField(prop)` to drop any pending debounced edit, and surfaces the toast via the dedicated `onVerdictFlip` callback (NOT `onError`). EntityDetail wires `onVerdictFlip` to a warning toast only; no loadView refetch (the loadView that surfaced the new verdict is what triggered this). The inverse `false → true` (permission restored) is intentionally silent — the cell simply becomes editable again on next render; no destructive UX consequence to warn about (round-3 N-R3-1). Tests cover both directions.

7. **Optimistic-section / non-optimistic-siblings model (RR-FB1C / RR-FB1N).** The edited cell shows the new value immediately (v-model bound to formData). Other sections that read `entry.properties[prop]` show the old value until `onPropertyApplied` fires (~800-1500ms after edit). No header Badge exists, so no header inconsistency. AutoSaveIndicator gives visible feedback. Documented in deltas table (delta #5).

8. **Existing tests unchanged.** All current `useAutoSave`, `DynamicForm`, EntityDetail content-channel, e2e `checkboxes.spec.ts`, and per-widget tests pass without modification.

9. **`SectionEditForm` unit tests (RR-UE3D + RR-FB1L + RR-FB1M + RR-FB2A + RR-FB2C + RR-FB2D):**
   - Mount with two fields, one writable / one not; assert writable cell hosts `<FieldShell>` wrapping `mode='edit'` widget, non-writable cell renders `mode='display'` widget.
   - `update:modelValue` on writable widget → `useAutoSave.scheduleFieldSave` invoked with matching `(prop, value)`.
   - Cleared writable value → `useAutoSave.scheduleUnset` invoked, not `scheduleFieldSave` (RR-FB1E).
   - `applyServerProperty(prop, undefined)` → deletes formData[prop]; `onPropertyApplied` invoked with `undefined` (RR-FB2D NEW-5).
   - `applyServerProperty(prop, value)` → `onPropertyApplied` invoked with `(prop, value, owner)` where `owner.type === props.entityType` and `owner.id === props.entityId` (RR-FB2A).
   - `onPropertyApplied` throws → caught, `onError` NOT invoked from this path (the catch logs to console only), formData reflects server value (not rolled back per RR-UE3D).
   - `fields` prop change with `writable: true → false` on prop X while X has pending debounced edit → `revertField` invoked, `onVerdictFlip` invoked with `(X, label)`, `onError` NOT invoked, formData[X] reflects baseline (RR-FB1M + RR-FB2C).
   - `commitImmediately` is called from `onBeforeUnmount`.
   - 422 server response on field Y → `autoSave.fieldErrors[Y]` non-empty; FieldShell renders error pill at cell Y only (RR-FB1L).

10. **EntityDetail integration tests:**
    - Mount with fixture entry where `_fields` is absent → properties section renders `<SectionEditForm>` (all fields default-writable per `Entity._fields?` semantics; sectionHasAnyWritable returns `true` because each field's verdict is undefined → default writable).
    - Mount with fixture entry where `_fields: {}` (empty but present) → same as above (per Entity.ts comment: "empty means evaluated, no deviations").
    - Mount with `_fields: { status: { writable: false } }` and section has only `status` → renders `<PropertyDisplay>`.
    - Mount with `_fields: { status: { writable: false } }` and section has `[status, title]` → renders `<SectionEditForm>` with `status` non-writable and `title` writable.
    - Edit on writable field → debounce later PATCH fires → `onPropertyApplied(prop, value, owner)` writes back → spread-clone updates `viewData.entry.properties[prop]` → sibling sections re-render.
    - Route navigation A→B with pending debounced edit on A → `:key` change forces SectionEditForm remount → onBeforeUnmount flushes PATCH against entity A → A's response arrives LATER; `handlePropertyApplied` identity-guard REJECTS (owner=A, current=B) → B's view is NOT mutated (RR-FB2A).
    - 403 from server → loadView triggered once via pendingRefetch dedupe; 401 same path (RR-FB2D NEW-6).
    - Verdict-flip on next loadView → onVerdictFlip toast fires; NO loadView refetch (RR-FB2C).
    - useAutoSave.test.ts gains structured-onError tests for all three channels (RR-FB2D NEW-7).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: middle slice of an already-researched feature; constraints inherited from TKT-IHCY7's three rounds + IHC7B's round-1 review)
- [x] Searched for existing libraries — N/A (Vue 3 SFC + composable; not a third-party concern)
- [x] Checked codebase for similar patterns
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:**

- `DynamicForm.vue` — the canonical `useAutoSave` host. `SectionEditForm` is "a small DynamicForm for one section's properties". Reuses the same composable, same callback shapes; differs only in (i) no relations/content surfaces (channels disabled), (ii) no top-level form chrome, (iii) per-section AutoSaveIndicator, (iv) verdict-flip toast watcher. The cleared-value routing and the field-affordance helpers are EXTRACTED from DynamicForm into shared utils so both call sites use identical logic.
- `PropertyDisplay.vue` — current display-only renderer using `WidgetRoutingHint` + `mode='display'`. Stays as the read-only path; the `'properties'` section branches into SectionEditForm only when at least one field is writable.
- `useAutoSave.ts` — extended in TKT-IHC7A with `initialServerSnapshot`, `disable*Channel` flags. THIS ticket adds the structured `onError` info (RR-FB1K). No other API change.
- `WidgetRoutingHint` + `defaultRegistry.resolveFromHint` (UD7YR) — view-side widget routing. SectionEditForm's discriminated `fields` prop uses either the schema or hint path, never both.
- `FieldShell.vue` — error chrome reused for the writable-cell rendering (RR-FB1L).
- `useAutoSave.revertField` — already exists; SectionEditForm uses it for verdict-flip cleanup (no useAutoSave API addition needed).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach:**

1. **`useAutoSave.onError` signature extension (RR-FB1K).** In `composables/useAutoSave.ts`:
   - Change `AutoSaveOptions.onError` from `(msg: string) => void` to `(msg: string, info?: { status?: number; property?: string; channel?: 'property' | 'content' | 'relations' }) => void`.
   - Update the three call sites of `opts.onError` inside the composable (one per channel) to pass `{ status: info.status, property, channel: '...' }`.
   - Existing callers (DynamicForm L?, EntityDetail's contentAutoSave) ignore `info` automatically — JavaScript drops the second arg if not declared. No call-site changes needed.
   - Test: extend the existing useAutoSave.test.ts with a 403 case for property channel.

2. **Shared helpers (RR-FB1E + RR-FB1I).** Create:
   - `frontend/src/utils/affordances.ts` — `isFieldWritable`, `optionVerdictsFor`.
   - `frontend/src/utils/formValue.ts` — `isClearedForType`.
   - Refactor DynamicForm.vue L185-199 to call the new helpers. Existing DynamicForm tests must pass unchanged.

3. **`SectionEditForm.vue`** (~150 lines):
   - Script: precompute `widgetRows = computed(() => fields.map(f => ({ field: f, widget: f.kind === 'schema' ? defaultRegistry.resolve(undefined, f.propertyDef) : defaultRegistry.resolveFromHint(f.routingHint), writable: isFieldWritable(f.verdict), optionVerdicts: optionVerdictsFor(f.verdict) })))`.
   - `formData = reactive({ ...initialValues })` — local mirror.
   - `autoSave = useAutoSave({ ...channelDisabled, ...initialServerSnapshot, applyServerProperty: (prop, value) => { (formData as any)[prop] = value; try { onPropertyApplied(prop, value) } catch (e) { console.error(e) } }, onError: (msg, info) => emit/call onError(msg, info) })`.
   - `function onFieldUpdate(field, value) { if (isClearedForType(value, field.kind === 'schema' ? field.propertyDef : undefined)) autoSave.scheduleUnset(field.property); else autoSave.scheduleFieldSave(field.property, value) }`.
   - `watch(() => props.fields, (next, prev) => { /* detect writable flips per property; revertField + onError */ })`.
   - `onBeforeUnmount(() => { void autoSave.commitImmediately() })`.
   - Template: `dl.properties-list` (reuse PropertyDisplay's class for visual parity). For each `row of widgetRows`: `<dt>{{ row.field.label }}</dt> <dd> <FieldShell v-if="row.writable" :error="autoSave.fieldErrors[row.field.property]" :field-id="`field-${row.field.property}`"> <component :is="row.widget" mode="edit" :model-value="formData[row.field.property]" @update:model-value="(v) => onFieldUpdate(row.field, v)" :property-name="row.field.property" :property-def="row.field.kind === 'schema' ? row.field.propertyDef : undefined" :option-verdicts="row.optionVerdicts" /> </FieldShell> <component v-else :is="row.widget" mode="display" :model-value="formData[row.field.property]" :property-name="row.field.property" :property-def="row.field.kind === 'schema' ? row.field.propertyDef : undefined" /> </dd>`.

4. **EntityDetail integration:**
   - Add helpers `buildSectionEditFields(section, entry)` and `sectionHasAnyWritable(section, entry)` near `mapFieldsToProperties` (L403).
   - Add `handlePropertyApplied(prop, value)` doing spread-clone of viewData.
   - Add `handleSectionEditError(msg, info)` with the 403 → loadView once dedupe.
   - In the template `properties` branch, add `<SectionEditForm v-if="sectionHasAnyWritable(...)" :key="`${entry.type}/${entry.id}`" .../>` before the existing `<PropertyDisplay v-else-if=".../>`.

**Files to modify:**

- `frontend/src/composables/useAutoSave.ts` — `onError` info extension; the three `opts.onError` call sites.
- `frontend/src/composables/useAutoSave.test.ts` — onError-info test cases.
- `frontend/src/utils/affordances.ts` — NEW.
- `frontend/src/utils/affordances.test.ts` — NEW.
- `frontend/src/utils/formValue.ts` — NEW (or wherever isClearedForType ends up; check if there's already a formValue utility module).
- `frontend/src/utils/formValue.test.ts` — NEW (or extend existing).
- `frontend/src/components/forms/DynamicForm.vue` — refactor L185-199 to use shared helpers.
- `frontend/src/components/forms/SectionEditForm.vue` — NEW.
- `frontend/src/components/forms/SectionEditForm.test.ts` — NEW.
- `frontend/src/components/entity/EntityDetail.vue` — properties-section branch + helpers + handlers.
- EntityDetail tests if any cover the properties section.

**Alternatives considered:**

- *(a) Widen `WidgetMode` to add `'inline-edit'`.* Rejected (RR-FB1F). The only behavioural delta is SelectWidget's transitions panel; suppressing it by not passing `transitions` is far smaller-blast-radius than a contract-wide third state. UD7YR's "reserved for IHCY7" comment is left in place — accurate forward-looking, no removal needed.
- *(b) Mutate `entry.properties[p] = v` directly.* Rejected (RR-FB1G). Spread-clone matches EntityDetail's existing checkbox-toggle path post-IHC7A.
- *(c) Read `useAutoSave.lastSeenServer[prop]` for verdict-flip revert.* Rejected (RR-FB1A). `lastSeenServer` is private. `revertField` already exists and routes through `applyServerProperty`, which writes the server-truth value back. Use it.
- *(d) Split SectionEditForm into a composable controller + thin presenter.* Deferred (RR-FB1P). Defensible cleanup but adds scope; revisit if the SFC grows past ~200 lines or testing-without-DOM becomes painful.
- *(e) Provide/inject for the schedule function so slot consumers can wire widgets.* Rejected (RR-UE3A). Vue's `inject` runs in the calling component, not in a parent template; slot authors cannot inject across the slot boundary. The "SectionEditForm owns the iteration" design (RR-UE3C option c) sidesteps this entirely.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Widget `update:modelValue` payloads — trusted by the existing form path (server validates on PATCH). Reuses the existing PATCH route through `useAutoSave`. No new validation needed.
- `_fields[prop]` (FieldAffordance) — server-supplied affordance, treated as advisory UX gating. Server is authoritative; client-side gating is best-effort. A user who bypasses the client guard gets a 403 from the server, which routes through the new structured `onError` (RR-FB1K) → `handleSectionEditError` → toast + loadView refetch.
- Verdict cache vs. server reality — addressed by RR-FB1M's verdict-flip watcher; deny-list is enforced on the server, the client UI just keeps up.

**Security-Sensitive Operations:**

- Property PATCH submission — uses existing `entitiesStore.update` path with Origin/CSRF guards unchanged. No new attack surface.
- 401/403 handling — `useAutoSave.onError` now passes `{ status }`; SectionEditForm forwards to host; host triggers `loadView()` (deduped via `pendingRefetch` flag). Loop guard: only ONE re-fetch per 403; subsequent 403s within the same loadView cycle just toast.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** (see AC 9 and AC 10 above)

**Edge Cases:**

- All-non-writable section → `sectionHasAnyWritable` returns false → renders `PropertyDisplay`, no autosave instance instantiated.
- `_fields: undefined` on entry → every field defaults writable → renders `SectionEditForm`.
- `_fields: {}` on entry → "evaluated, no deviations" → every field defaults writable → renders `SectionEditForm`.
- Empty `fields` array (no properties in section) → renders zero rows, no autosave fire, no `<SectionEditForm>` instance (sectionHasAnyWritable returns false).
- `ViewSectionField.property === undefined` → filtered out by `buildSectionEditFields`; if no property-bearing fields remain, fall back to PropertyDisplay (RR-FB1J).
- Same-value edit (widget emits update with current baseline) → no-op suppression via TKT-IHC7A's `lastSeenServer` baseline (initialServerSnapshot seeded the values). No PATCH fires.
- Route navigation A→B with pending edit → `:key` change forces remount → previous instance's onBeforeUnmount flushes PATCH against A's props (Vue unmounts child before mounting new child; props on the unmounting child are still A's). New instance seeds against B.
- Two rapid edits to the same field → coalesced by the default 800ms debounce; only the latest value PATCHed.
- Edits to two different fields → FIFO chained via `queueTail` (existing useAutoSave behaviour).
- `onPropertyApplied` throws → caught, formData stays at server value (RR-UE3D). Toast not surfaced from this path (the server-confirmed value IS the server-confirmed value; rollback would inconsistently make the section show stale data).
- Server 422 (validation failure) on field X → `autoSave.fieldErrors[X]` populated → FieldShell renders error pill at X. Other cells unaffected.

**Negative Tests:**

- `useAutoSave` throws on `scheduleContentSave` (content channel disabled): asserted via existing TKT-IHC7A test.
- `useAutoSave` throws on `scheduleRelationsChange` (relations channel disabled): asserted via existing TKT-IHC7A test.
- 403 PATCH response on writable field → `onError` invoked with `{ status: 403, property, channel: 'property' }`; host re-fetches `loadView()` once.
- Verdict-flip mid-debounce → pending edit dropped via revertField, toast surfaced.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated — `m`

**Risks:**

- **Refactor of DynamicForm's affordance helpers may introduce subtle drift.** Mitigation: existing DynamicForm tests must pass unchanged; if any do not, the extracted helpers are not behavior-preserving and need fixing before merge.
- **`:key="entry.type/entry.id"` may have surprising remount behaviour for keep-alive or transitions.** Mitigation: EntityDetail today does not use `<KeepAlive>` or `<Transition>` around the properties section; new `:key` is local to one tag.
- **Per-cell error chrome via FieldShell may visually disrupt the `dl` layout.** Mitigation: SectionEditForm visual smoke test (browser), tweak FieldShell margins inside `<dd>` if needed. Small CSS-only follow-up if it lands wrong.
- **Verdict-flip race when PATCH is in flight at the moment of flip.** Mitigation: in-flight PATCH completes; server arbitrates; `applyServerProperty` writes the server-truth value (whether the PATCH succeeded or was rejected). `lastSeenServer` baseline is updated via `mergeServerResponse`. revertField on the (now-flipped) prop is a no-op if there's nothing pending.
- **Re-fetch loop on persistent 403.** Mitigation: `pendingRefetch` flag in EntityDetail prevents repeat loadView calls per-error.

## Documentation Planning

- [x] User-facing docs identified — N/A
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: internal Vue SFC + composable wiring; the new affordance/cleared-value utility modules get jsdoc at the source)

**Documentation Impact:** N/A — internal change. The user-visible behaviour
change (inline-edit of writable properties; verdict-flip toast on permission
change) is self-evident in the UI.

## Design Review

- [x] Run `/design-review` before starting implementation — rounds 1 & 2 complete; 21 findings captured as RR-FB1A..RR-FB1Q + RR-FB2A..RR-FB2D
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 21 total findings across two review rounds.
- Round 1: 17 findings (RR-FB1A..RR-FB1Q). 5 critical, 7 significant, 2 minor, 3 nit.
- Round 2: 4 findings rolling up 10 new concerns the rewrite introduced (RR-FB2A..RR-FB2D). 1 critical (response-merge race on `:key` remount, NEW-1), 2 significant (helper-extraction regression NEW-2 + verdict-flip conflation NEW-3), 1 minor (NEW-4..NEW-10 rolled up).
- All critical + significant addressed. RR-FB1P (controller/presenter split) deferred to follow-up.

| Finding | Severity | Status | Disposition |
|---|---|---|---|
| RR-FB1A | critical | addressed | Use `viewData.entry.properties` + `revertField`; no `lastSeenServer` exposure |
| RR-FB1B | critical | addressed | `_props` references deleted |
| RR-FB1C | critical | addressed | AC 5 rewritten to "next loadView refreshes all consumers"; no header Badge |
| RR-FB1D | critical | addressed | `:key="entry.type/entry.id"` on SectionEditForm; remount handles route changes |
| RR-FB1E | critical | addressed | Extract `isClearedForType` into `utils/formValue.ts`; SectionEditForm uses it |
| RR-FB1F | significant | addressed | Drop WidgetMode widening; SectionEditForm doesn't pass `transitions` |
| RR-FB1G | significant | addressed | Spread-clone in handlePropertyApplied (matches IHC7A pattern) |
| RR-FB1H | significant | addressed | Discriminated union on `fields` prop; no bang-casts |
| RR-FB1I | significant | addressed | `verdict?: FieldAffordance` carries options + writable; shared helper extracted |
| RR-FB1J | significant | addressed | Filter `f.property !== undefined` before iterating |
| RR-FB1K | significant | addressed | `useAutoSave.onError` extended with structured info |
| RR-FB1L | significant | addressed | Reuse FieldShell for per-cell error chrome |
| RR-FB1M | significant | addressed | SectionEditForm watches `fields` for writable flips |
| RR-FB1N | significant | addressed | "Section optimistic, siblings reconcile" model documented; no header inconsistency |
| RR-FB1O | nit | addressed | N1-N6 dissolve via other resolutions; PR #912 merge confirmed |
| RR-FB1P | nit | deferred | Controller/presenter split — defer to follow-up |
| RR-FB1Q | minor | addressed | Widget count + framing — moot after RR-FB1F |
| RR-FB2A | critical | addressed | Owner identity in onPropertyApplied; handlePropertyApplied identity guard |
| RR-FB2B | significant | addressed | isFieldWritable takes fieldReadonly; preserves DynamicForm's static-readonly channel |
| RR-FB2C | significant | addressed | Separate onVerdictFlip callback; no spurious loadView refetch |
| RR-FB2D | minor | addressed | NEW-4 memoize; NEW-5 undefined-as-delete; NEW-6 401 also refetches; NEW-7..10 minor sharpenings |
