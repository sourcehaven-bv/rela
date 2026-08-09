---
id: PLAN-B4481K
type: planning-checklist
title: 'Planning: Inline entity creation from relation fields in the data-entry edit form'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Revised after `/design-review`** — see RR-KGCF61, RR-R1I6AD (critical),
> RR-Y15LRA, RR-UTURB1, RR-PP3B60, RR-P3CO33, RR-BEMW01 (significant),
> RR-3UOH1I (minor). The endpoint design, condition B's definition, AC3, and
> the migration scope all changed as a result.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** From an entity form, a user who needs to link a target that does
not exist yet must abandon the form (destroying the draft — create-mode state is
component-local with no persistence), create the target on its own page, then
navigate back and start over.

**The rule.** Inline create is offered for a target type iff:

- **A — permission**: the principal may `create` that entity type; and
- **B — a usable form exists**: `createFormForType` resolves a form for it.

Note B follows the **existing resolver's** definition (`views_handler.go:684`):
prefer a non-edit form, **fall back to an edit-mode form**, which its doc
comment notes "work for creation when no entity ID is provided". Condition B is
therefore "a form exists", not "a non-edit form exists" (RR-KGCF61).

**Scope — IN:**

- A modal hosting a real nested create form, driven by the same
`data-entry.yaml` form definition and `DynamicForm` machinery as a top-level
create form (fields, widgets, templates, validation, wizard steps, dry-run
affordances).
- The derived offer rule above, with a **structurally enforced** one-level cap.
- Surfacing condition A (per-type create permission) on a principal-scoped
payload. Condition B is derived **client-side** from `_config`, which already
ships every form with `entity_type` and `mode` (RR-R1I6AD).
- The affordance in **both** relation widgets: `RelationPicker` (replacing
today's button) and `RelationCards` (new, feeding `selectTarget` so edge-meta
can still be filled before linking).
- Deleting `FormRelation.AllowCreate` / `FormRelation.CreateForm` (Go + TS +
validation). **No migration** — see below.
- Deleting `InlineCreateModal.vue`.
- Unit + e2e coverage, replacing the vacuous e2e placeholder.

**Scope — OUT:**

- Nested create beyond one level — now *enforced* by an injected depth, not
merely documented (RR-UTURB1).
- A `rela migrate` step. Verified unnecessary: `checkUnknownKeys`
(`validate.go:149-168`) validates **top-level keys only**, and nested struct
unmarshal is non-strict `yaml.Unmarshal`, so a stray `allow_create:` under a
form relation is silently ignored rather than an error. No form relation in the
repo sets either key (all 16 `create_form:` hits in `tickets/data-entry.yaml`
are on lists/kanbans, which are kept) (RR-Y15LRA).
- A focus trap for modals — separate backlog ticket `TKT-X4P99`.
- Draft persistence for form state.
- Parent-context `visible_when` conditions in a nested form (RR-3UOH1I).
- The two unguarded list create affordances found during research
(`EntityList.vue:778` empty-state link, `:211` `N` shortcut) — real but
pre-existing bugs; file separately.
- Deriving `create_form` for lists/kanbans (they intentionally require explicit
config).

**Acceptance Criteria:**

1. **AC1 — Offer appears when permitted and a form exists.** A relation whose
target type resolves a form, for a principal with create permission, shows a `+
New <Label>` affordance in both `RelationPicker` and `RelationCards`.
2. **AC2 — Offer hidden without permission.** With an ACL role lacking
`create: [targetType]`, the affordance is absent, even though a form exists.
3. **AC3 — Offer hidden when no form resolves.** For a target type with **no
form at all**, the affordance is absent. A type with only an **edit-mode** form
*does* get the affordance and uses that form — matching the resolver
(RR-KGCF61).
4. **AC4 — The nested form is the configured form.** The modal renders exactly
the fields/relations/steps of the resolved form definition — not the raw
metamodel property list — including templates. `visible_when` evaluates against
the **nested** form's own data (parent-context conditions are unsupported and
documented as such).
5. **AC5 — Create-and-link round trip.** Submitting the nested form creates the
entity and selects it in the parent widget; saving the parent persists the edge.
For `RelationCards`, the created entity lands in `selectTarget` so required
edge-meta can be filled before Link.
6. **AC6 — The parent draft survives.** Opening, submitting, and cancelling the
modal leaves every parent field, relation selection, wizard step and body
content untouched. No navigation occurs.
7. **AC7 — No cross-form interference.** Cmd/Ctrl+Enter inside the modal submits
only the nested form; a nested wizard does not disturb the parent's `?step=`; an
unsaved-changes confirm from one form never silently answers for the other.
8. **AC8 — `false` booleans persist.** A boolean field explicitly set false in
the nested form is sent and stored (the bug in the removed modal).
9. **AC9 — Validation and warnings.** Required-field validation blocks submit
with per-field errors; server warnings surface.
10. **AC10 — Nesting is capped structurally.** A `RelationPicker` inside a
nested form offers **no** inline-create affordance (RR-UTURB1).
11. **AC11 — create-without-read degrades gracefully.** A principal who may
create but not read a type can create and link it in-session; after a re-fetch
404s, the card renders the entity **id** — never blank, never an error toast
(RR-PP3B60).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — targeted codebase surveys plus an adversarial design
review were sufficient; no external library question (this is entirely internal
component composition).

**Existing Solutions:**

- **Libraries:** none applicable. A third-party modal/form library would
duplicate `DynamicForm`.
- **The derivation pattern:** `createFormForType` (`views_handler.go:684`)
already resolves "the create form for entity type X". Its single caller
`sections.go:394` builds side-panel `SectionAddTarget`s exactly this way. Reused
**unchanged**; this feature adds the second consumer (client-side).
- **The permission signal:** `computeCollectionActions`
(`internal/dataentry/affordances.go:135`) already answers `OpCreate` against
`EntitySubject{Type: X, ID: ""}`, emitted as `_actions.create` on list responses
(`api_v1.go:653`). Reused verbatim.
- **The affordance wire contract:** sparse deny-only maps with `actionAllowed()`
(`frontend/src/utils/affordancesWarning.ts:33`) failing open on `undefined`.
- **Modal hosting:** `InlineCreateModal.vue` is the only precedent and is what
we remove. `EntityPickerModal.vue` is the best a11y reference. `modalStack.ts`
exists for shortcut suppression — but is a `Set`, not a stack (it answers "is
any modal open", not "am I topmost"), which is why nesting is prevented outright
rather than coordinated.
- **Rejected alternative found in tree:** `LinkExistingModal` was deleted under
TKT-651W — the project prefers fewer bespoke modals.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Condition B — client-side, zero server change.*

`_config` already serves the whole `Forms` map, each with `entity_type` and
`mode` (`responses.go:256`). The SPA derives the create form per type by
mirroring `createFormForType`'s ordering: natsort form ids, take the first
non-edit form for the type, else the first edit-mode one. A Go test pins that
ordering as the contract the client replicates.

Crucially `_config` is **pinned principal-independent** by
`nav_permission_test.go:346` — so it can carry the form data but must not carry
the ACL boolean.

*Condition A — the only new server value.*

A per-type `create` permission map, computed with `computeCollectionActions`
verbatim, served on a principal-scoped read (the sidebar/affordance payload,
which already varies per principal). **Not** on `_schema`: that handler builds
from `a.State().Meta` alone (`api_v1.go:1148`), has no access to the
config-layer resolver, and is a pure metamodel projection — putting config- or
principal-derived data there is a layering violation (RR-R1I6AD).

The SPA caches this for the session (`schema.ts:216` `if (loaded.value)
return`). Accepted and documented: `_actions` is a UI hint the POST
re-authorizes, so a stale map can only show a button that then 403s — it can
never grant anything.

*Config — delete the dead knobs.*

Remove `AllowCreate` / `CreateForm` from `dataentryconfig.FormRelation`
(`config.go:202-203`), the `create_form` branch in `validate.go:359`, and the
mirrored TS fields. No migration (see Scope — OUT).

*Frontend — make `DynamicForm` embeddable.*

Add `embedded?: boolean` and `defineEmits<{ 'inline-created': [Entity];
'inline-cancelled': [] }>()`. Names are deliberately namespaced: declaring emits
removes those names from `$attrs` and changes fallthrough, so generic
`created`/`cancel` would be a trap for a future mount expecting a native
listener (RR-P3CO33).

Guard on `embedded`:

| Site | Line | Guard |
| --- | --- | --- |
| `document` keydown (Cmd+Enter) | 1164 | don't register; modal owns the shortcut |
| `onBeforeRouteLeave` | 1285 | **called at setup time inside `if (!props.embedded)`** — conditional registration, not a runtime branch. `embedded` is never reactive after mount. |
| `route.query` reads (`return_to`, `prop.*`, `rel.*`, `link_*`) | 1167, 388-412 | skip; modal passes explicit initial values |
| `router.push` after create | 939-943 | `emit('inline-created', entity)` and return |
| `handleCancel` `router.back()` | 970 | `emit('inline-cancelled')` |
| page `<header>` | 1323 | `v-if="!embedded"` — modal supplies its own |
| success toast | 935 | suppress; the modal's own message is the signal |

Thread `syncUrl: false` into `useFormWizard` so a nested wizard doesn't fight
over `?step=` (`useFormWizard.ts:211` plus the seed/watch pair).

*Frontend — the modal shell.*

New `InlineCreateFormModal.vue`: `Teleport to="body"`, `useModalStack`, props `{
show, formId, entityType }`, emits `{ close, created }`, hosting `<DynamicForm
:form-id="formId" embedded />` under **`v-if`, never `v-show`** — `DynamicForm`
*awaits* `refreshStagedAffordances()` on mount (`:1187-1190`, the F19 no-flash
guarantee) and re-runs a dry-run POST debounced at 400ms, so the modal shows its
own loading state while that resolves, and unmounting on close is what fires the
existing `stagedUnmounted` / `stagedDryRunController` abort path (RR-BEMW01).
Two concurrent dry-run loops are accepted: both are debounced, abortable and
read-shaped (no `writeMu`, no audit row).

The modal scopes its own width (`.dynamic-form` is `min-width: 500px`) and must
not inherit `.form-layout.with-sidepanel`. Owns Escape and Cmd+Enter. It
provides `inlineCreateDepth` = parent depth + 1.

On success it reports **"Created <title> — link it below"** rather than closing
silently, so the user knows the entity exists even if they then cancel the Link
step (RR-3UOH1I).

*Frontend — the offer.*

`useInlineCreate(targetTypes)` injects `inlineCreateDepth` (default 0) and
returns **nothing at depth >= 1**, making modal-in-modal unreachable rather than
merely discouraged (RR-UTURB1). Otherwise it returns, per target type, `{
entityType, label, formId }` for types where a form resolves and the create
permission map is not `false`. Then:

- `RelationPicker`: replace the `+ Add new` buttons (`:376-388`) with entries
from the composable; `handleEntityCreated` (`:286`) already handles both
directions and needs no change.
- `RelationCards`: add a parallel `+ New <Label>`; the created entity flows into
`selectTarget(entity)` (`:312`) so required edge properties can be filled before
`addRelation()`.

Today's button is gated on `verdict.creatable` — the *relation-edge* permission.
That gate stays and is AND-ed with the new entity-create gate; they are
orthogonal.

**Alternatives considered:**

- *Keep a bespoke lighter modal driven by form config* — rejected: it
re-implements validation, templates, wizard steps and dry-run affordances, which
then drift from `DynamicForm`. That drift is the current bug.
- *Boolean `allow_create` config gate* — rejected: three of its four states with
`create_form` are degenerate, and a form definition is already the natural
"which fields" declaration.
- *Put both signals on `_schema`* — rejected after review (RR-R1I6AD):
layering violation, handler has no access to the resolver, and `_config` already
carries the form data.
- *Derive permission from read visibility / sidebar* — rejected and dangerous:
`create` implies no `read` (`policy.go:239-243`).
- *Route to a real create page and return* — rejected: destroys the parent draft.

**Files to modify:**

- `internal/dataentry/` — per-type create permission map on the principal-scoped
payload, via `computeCollectionActions`
- `internal/dataentry/views_handler_test.go` — pin `createFormForType`'s three
shapes (create-only / edit-only / both) so the fallback can't be "fixed" out
from under `sections.go:394`
- `internal/dataentryconfig/config.go` — delete `AllowCreate`, `CreateForm`
- `internal/dataentryconfig/validate.go` — delete the `create_form` branch (:359)
- `frontend/src/types/config.ts`, `frontend/src/types/entity.ts`
- `frontend/src/stores/schema.ts` — per-type create-form derivation + permission
- `frontend/src/composables/useInlineCreate.ts` — new (with depth cap)
- `frontend/src/composables/useFormWizard.ts` — `syncUrl` option
- `frontend/src/components/forms/DynamicForm.vue` — `embedded` prop + emits
- `frontend/src/components/forms/InlineCreateFormModal.vue` — new
- `frontend/src/components/forms/InlineCreateModal.vue` — **delete**
- `frontend/src/components/forms/RelationPicker.vue`, `RelationCards.vue`
- `e2e/pages/form.page.ts`, `e2e/tests/forms.spec.ts`, `relation-cards.spec.ts`
- `docs/data-entry.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
| --- | --- | --- | --- |
| `formId` for the nested form | Derived from the `_config` forms map by the same rule the server uses; never free-text from the user | Must resolve to a form whose `entity_type` is a declared endpoint of the relation | No affordance |
| Nested form field values | User | Same client validation as a top-level create, then the **server re-validates and re-authorizes** on `POST /api/v1/{plural}` | 400/422 surfaced per field |
| Target entity type | Relation def `to:`/`from:` in the metamodel | Constrained to the relation's declared endpoints | Not offered |

**Security-Sensitive Operations:**

- **Authorization is not moved to the client.** The per-type create map is a UI
hint; `handleV1CreateEntity` (`write_handler.go:88`) performs the real
`OpCreate` check and the BUG-Q60V field-affordance gate unchanged. Hiding the
button is UX, not enforcement — an unauthorized POST still 403s. This is the
documented `_actions` contract (`api-reference.md:344`).
- **No new endpoint and no new capability.** The one new value is already
computed by an existing function.
- **`_config` stays principal-independent** — pinned by
`nav_permission_test.go:346`. Nothing principal-varying is added to it. Form
names are not secrets (project rule) and `_config` already serves them all.
- **The permission map is principal-scoped** and must be tested as such: two
principals, different booleans, no cache bleed. The SPA's session cache is
documented as fail-safe (stale ⇒ a button that 403s, never a grant).
- **create-without-read is not a leak**: the only id shown back is one the
principal itself just authored and linked (RR-PP3B60).
- **Error handling:** creation failures surface the existing server message. A
denied create returns the standard 403 without enumerating roles.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Level |
| --- | --- | --- |
| AC1 | Permission map + resolvable form → both widgets render `+ New` | Go handler test + Vitest per widget + e2e |
| AC2 | ACL fixture role without `create: [feature]` → map says false → no affordance | Go ACL test + Vitest |
| AC3 | Type with **no** form → no affordance. Type with **only an edit-mode** form → affordance present, uses that form. Plus the `createFormForType` three-shape table test | Go + Vitest |
| AC4 | Modal renders the configured form's fields, not the metamodel list; a field absent from the form does not appear; a template applies; `visible_when` reacts to nested data | Vitest + e2e |
| AC5 | Picker: create → selected → parent save PATCHes the edge. Cards: create → `selectTarget` → meta filled → Link → `cards-changed` carries it | Vitest + e2e both widgets |
| AC6 | Fill parent fields + a wizard step, open modal, submit, assert every parent value and step index unchanged; repeat with cancel | e2e (highest value) |
| AC7 | Cmd+Enter in modal creates exactly one entity; nested wizard leaves `?step=` alone; no route guard registered when embedded | Vitest + e2e |
| AC8 | Boolean set explicitly false round-trips to the store | Vitest + e2e |
| AC9 | Empty required field blocks submit with per-field error; server warning surfaces | Vitest + e2e |
| AC10 | `useInlineCreate` returns an empty list when `inlineCreateDepth` is injected as 1 | Vitest |
| AC11 | `getEntity` stubbed to reject 404 → card renders the id, no error toast | Vitest |

**Integration approach:** the load-bearing tests are e2e (Playwright), because
the risk is *interaction between two mounted form instances* — not reproducible
with a mocked router. Page objects `hasInlineCreateButton()` /
`clickInlineCreateButton()` / `expectInlineFormVisible()` in
`e2e/pages/form.page.ts` must be rewritten to match real markup (they currently
match nothing, which is why the existing test is vacuous). `router_walk_test.go`
needs no new route.

**Edge Cases:**

- Relation with **multiple target types** → one `+ New <Label>` per *eligible*
type; ineligible types silently omitted; zero eligible → no affordance at all
(not an empty menu).
- **Incoming-direction** relation → candidates from `relationType.from`;
`handleEntityCreated` must still emit the incoming diff (regression risk against
BUG-10IPBP, pinned by `incoming-picker-create-persist-test`).
- Nested form whose resolved form is a **wizard** → steps render in the modal
without touching the parent's `?step=`.
- Nested form's own relation fields → `RelationPicker` only (cards need
`entityId`), and **no** inline-create offer (AC10).
- Created entity **outside the picker's 100-candidate window** → still
selectable (today's `candidates.push` handles this; keep it).
- **Orphan on Link-cancel**: create → `selectTarget` → `cancelAdd()`
(`RelationCards.vue:373`) leaves the entity persisted and unlinked. Likelier
than a save failure; the modal's "Created — link it below" message makes it
visible rather than silent.
- **Orphan on parent-save failure**: entity persists unlinked. Acceptable
without a transaction; must not read as data loss.
- Modal open while an **SSE refresh** lands → parent must not refetch over the
draft (`dirtyFormRegistry` covers edit mode).
- Entity type with **manual id** or multi-prefix → nested form carries the id
controls (`useEntityIDControls`), which `DynamicForm` already does.
- **Escape** with a dirty nested form → confirm discard; must not propagate to
the parent or trigger its route guard.
- **Mobile viewport** → `.mobile-actionbar` fixed positioning must not escape
the modal.

**Negative Tests:**

- POST from a principal lacking create permission → 403 even with the button
forced visible (proves the gate is not client-side).
- A type whose form resolves to `""` → no affordance, rather than a modal with
an empty form.
- Nested form with a required field empty → blocked client-side; with client
validation bypassed → 422 surfaced per field.
- Two principals reading the permission map → different values, no cache bleed.
- `getEntity` 404 after create-without-read → graceful id fallback (AC11).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Severity | Mitigation |
| --- | --- | --- |
| `DynamicForm` is 1877 lines; adding `embedded` touches a component every form depends on | **High** | Prop defaults to false. The only existing mount (`FormView.vue:11`) passes no listeners and no attrs, so it is unaffected — the `defineEmits` fallthrough change is real but unobservable today, and the namespaced emit names keep it that way. Existing `DynamicForm.test.ts` / `.guard.test.ts` must pass unchanged. |
| Two mounted forms interfering (global keydown, route guard, `useConfirm` singleton, wizard URL, dry-run loops) | **High** | Each individually disabled by `embedded` (table above); each gets a dedicated e2e assertion under AC7. Primary reason e2e is the main level. |
| Someone "fixes" `createFormForType`'s edit fallback and breaks side panels | **Medium** | Three-shape table test pins current behaviour before any change (RR-KGCF61). |
| Permission map bleeding across principals | **Medium** | Two-principal test; session cache documented as fail-safe. |
| Deleting `allow_create`/`create_form` | **Low** | Provably read by nothing, set by no config in-repo, and unknown nested keys are silently ignored — no migration needed. |
| Today's `+ Add new` disappears where no form resolves | **Low** | Intended — it currently opens the broken modal. Changelog note. Consistent with lists/kanbans. |
| Modal layout (`min-width: 500px`, `.with-sidepanel`, mobile actionbar) | **Low** | Modal scopes its own width; mobile viewport covered in e2e. |
| No focus trap in any modal | **Low** | Pre-existing, tracked by `TKT-X4P99`; don't regress keyboard behaviour. |

**Effort:** m — the server change is now minimal (one already-computed value);
work and risk are concentrated in making `DynamicForm` embeddable and proving
non-interference.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — inline creation; the derived rule (permission ∧ a
resolvable form); that an edit-mode form is used as a fallback; that a small
dedicated create form yields a small inline modal; that `visible_when` in a
nested form sees only its own data; and the removal of `allow_create` /
`create_form` from form relations.
- [x] Changelog — `allow_create` / `create_form` on form relations are removed
(silently ignored if left in YAML); the inline-create button now requires a
resolvable create form and create permission for the target type.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no command change)
- [x] ~~`CLAUDE.md`~~ (N/A: follows existing affordance/consumer-side patterns, no new rule)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-KGCF61 (critical), RR-R1I6AD (critical),
RR-Y15LRA (significant), RR-UTURB1 (significant), RR-PP3B60 (significant),
RR-P3CO33 (significant), RR-BEMW01 (significant), RR-3UOH1I (minor) — all
addressed above.
