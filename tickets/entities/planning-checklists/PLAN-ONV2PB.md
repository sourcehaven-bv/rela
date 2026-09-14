---
id: PLAN-ONV2PB
type: planning-checklist
title: 'Planning: Create forms need a "Create & add another" button for repeated entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Entity create forms in the data-entry SPA commit exactly once and then navigate
away (`router.push` to the entity detail page, or to `return_to`). Entering a
batch of entities of the same type therefore costs a full
list → "+ New" → fill → Create → navigate round trip per record. `e2e/tests/create-redirect.spec.ts`
literally loops `navigateToCreateForm` three times to enter three features —
that loop is the cost this ticket removes.

The form is already single-use *by construction*: `createdEntityId` (RR-4QO887,
`DynamicForm.vue:150`) latches on a successful create and makes every later
submit a no-op, precisely because the happy path navigates away and the failure
path must not mint a duplicate. "Create & add another" is therefore not just a
button — it is a second, explicit terminal outcome for the create path that
resets the form instead of navigating, and the latch has to become
outcome-aware rather than simply being removed.

**Scope:**

IN scope:

- A secondary "Create & add another" action on the **create** form, beside the
  existing primary "Create" button (`DynamicForm.vue:2041-2051`).
- A **clean** reset-to-fresh-create path built on the existing
  `initializeDefaults()` + `applyTemplate()` + `refreshStagedAffordances()`
  machinery rather than inventing a second notion of "a blank form".
- **A new `data-entry.yaml` config option marking individual fields/relations as
  carrying over to the next record** (operator decision, per field). Everything
  not so marked is cleared. This spans `internal/dataentryconfig` (struct tag +
  semantic validation), the JSON the API already serves, the TS types, and the
  reset itself.
- Success confirmation naming the entity that was just created (the user is no
  longer taken to it, so the toast is the only evidence it exists).
- Correct interaction with the one-shot `createdEntityId` latch, the `dirty`
  flag / unsaved-changes route guard, and the wizard step.
- Unit tests (`DynamicForm.*.test.ts`) and one e2e spec.

OUT of scope:

- **Edit forms.** Edit mode autosaves per field and `handleSubmit` early-returns
  for it (`DynamicForm.vue:1050`); there is no submit button to sit beside.
- **Embedded / inline-create forms** (`props.embedded`, `InlineCreateFormModal.vue`).
  Their contract is "create one entity, hand it to the host, host closes the
  modal and links it" (`DynamicForm.vue:1183-1187`). "Add another" has no
  meaning when the host is waiting for exactly one entity to link.
- Keyboard shortcut for the new action. `Cmd+Enter` stays bound to the primary
  Create (`DynamicForm.vue:1622`). Adding a second chord is separate work.
- Bulk/CSV import, a multi-row grid entry mode, or any "enter N at once" UI.
- Changing what the primary Create button does.
- **A pre-existing prefill mismatch found while researching, unrelated to this
  ticket:** `SidePanel.vue:71-80` pushes `_relation` / `_linkAs` / `_peerId`
  query keys, but `initializeDefaults()` reads `link_relation` / `link_peer` /
  `link_as` (`:611-612`) — so that path's prefill appears not to apply. This
  ticket neither depends on nor fixes it; **file it as a separate bug** rather
  than folding it in, since it would change behavior the acceptance criteria
  here do not cover.

**Acceptance Criteria:**

1. **The button exists on create forms only.**
   Test: mount `DynamicForm` with no `entityId` → a "Create & add another"
   button is present. Mount with an `entityId` (edit) → absent. Mount with
   `embedded: true` → absent.

2. **It creates the entity, with a payload identical to the primary Create's.**
   Including relations, id controls, staged attachments, and the `as === 'from'`
   auto-link `createRelation` (`:1146-1158`), which fires per record.
   Test (RR-7J1IHR): drive **both** buttons in ONE test against the same filled
   form and `expect(callA).toEqual(callB)`. Two separate tests could only
   compare against duplicated literals, which drift and would not actually test
   the "the two buttons cannot diverge" invariant. E2e asserts the entity is
   listed afterwards.

3. **It does not navigate.** After success the **path** is unchanged — still
   the create route. (Not "the query string is intact": AC-9 requires `?step=`
   to change on a wizard form, so the stronger claim is false — RR-7J1IHR.)
   Test: unit test asserts `router.push` was NOT called; e2e asserts
   `toHaveURL` still matches the create form path.

4. **The form is reset CLEAN.** User-entered property values, relation
   selections, body content, validation errors and staged files are cleared;
   metamodel defaults, form-level defaults, and `prop.*` / `rel.*` / `link_*`
   query pre-fills are re-applied; the chosen template is re-applied (without
   a refetch, and without reverting the user's template pill — RR-RV7WHL).
   Test: unit test fills a field, clicks the action, asserts the field is back
   to its default and a `?prop.status=open` pre-fill is still applied. The
   value half is also covered by a mount-free `blankCreateState` unit test.

4b. **A field or relation marked `keep_on_add_another: true` carries its value
   over; an unmarked one does not.** A kept value takes precedence over the
   default that would otherwise be restored.
   Test: form config with one marked and one unmarked field plus one marked
   relation; fill all three, use the action, assert the marked two retain the
   entered values and the unmarked one is back to its default. Second test:
   a marked field whose metamodel default differs from the entered value keeps
   the *entered* value, proving order (capture → clear → defaults → re-apply).

4d. **An incoming-direction `RelationPicker` does not carry its selection into
   record two (RR-VVLNSU).** The regression test for the duplicate-link write:
   the picker holds its own `incomingValue` (`RelationPicker.vue:88`), so
   clearing the parent's `relations` does not reach it.
   Test: create form with a `direction: incoming` relation field; select a
   peer, use the action, assert the picker is empty and that the second
   create's relation payload does not contain the first record's peer.

4c. **The config key round-trips and is not silently dropped.** A
   `data-entry.yaml` carrying `keep_on_add_another: true` on a field and on a
   relation loads without error and the value is readable on the parsed
   `Form` — the regression test for the `FormRelation.Span` failure mode (a
   missing struct tag would make this pass vacuously only if it asserted
   absence, so it must assert the value is `true`).
   If the `hidden`-field coherence check ships, one more case: it is a load
   error naming the form and field.
   Test: `internal/dataentryconfig/validate_test.go` / `config_test.go`,
   table-driven beside the existing `clear_when_hidden` cases.

5. **A second entity can immediately be created from the same form instance.**
   The `createdEntityId` latch must not block it.
   Test: unit test clicks the action, fills a different title, clicks it again,
   asserts `createEntity` called twice with two different titles. This is the
   regression test for the latch interaction.

6. **Success is confirmed and names the created entity.** A success toast
   reports the created entity id, since the user does not land on it.
   Test: asserts `uiStore.success` called with a message containing the id.

7. **The form is not left dirty** — including after an entry-locked field's
   dry-run resolves, which `adoptLockedFieldValues` (`:749`) would otherwise
   make dirty with nothing the user did (RR-1LMJSQ).
   Test: a test that MOUNTS the component and reads `defineExpose`'s
   `isDirty()` (`:1835`), as `DynamicForm.attachments.test.ts:285-295` already
   does. **Not** `DynamicForm.guard.test.ts` — that file never mounts
   `DynamicForm`; `makeFormHarness` (`:19-36`) is a hand-copied replica of the
   guard, so extending it would prove nothing about this path.

8. **Failure is not treated as success.** On a create error, the form is NOT
   reset (the user's input survives), no toast claims success, and validation
   errors render as they do for the primary Create.
   Test: unit test with a rejecting `createEntity` asserts form values intact
   and no reset.

9. **A multi-step (wizard) create form returns to step 1 after reset.**
   Test: unit test with a 2-step form config — advance to step 2, use the
   action, assert `wizard.currentStep` is 0 and `?step=` reflects it.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — a single-component UI change with a clear precedent;
the approach fell out of reading `DynamicForm.vue`. No competing options
worth a RES entity.

**Existing Solutions:**

- *Libraries:* none applicable. This is ~40 lines in one Vue component; a
  dependency would be absurd.
- *Prior art (external):* "Save and add another" is the standard pattern in
  Django admin, Rails/ActiveAdmin scaffolds, and Salesforce ("Save & New").
  All three share the same semantics adopted here: commit, stay on the form,
  reset to defaults, confirm inline. Django additionally preserves nothing but
  the URL context, which matches AC-4.
- *Prior art (in repo):* none —
  `grep -rn "add another\|addAnother\|saveAndNew\|createAnother"` over
  `*.ts`/`*.vue`/`*.go` returns only an unrelated `FileWidget.vue:269` string
  and prose in `docs/data-entry.md`. This is genuinely new behavior.
- *Reusable code found (this is what makes the ticket small):*
  - **`selectTemplate()` (`DynamicForm.vue:792-803`) is a working partial reset
    already in the file** — it clears `formData`/`relations`/`content`, calls
    `initializeDefaults()`, then `applyTemplate()`. It is the direct precedent
    for the reset and proves the approach; `resetCreateForm()` generalizes it
    (adding the latch, errors, staged files, wizard step and affordances) and
    `selectTemplate` should not grow a second, divergent copy.
  - `initializeDefaults()` (`DynamicForm.vue:583-676`) already re-derives the
    entire blank-create state — it calls `idControls.reset()`, applies
    metamodel property defaults, then form-level field/relation defaults, then
    `prop.*` / `rel.*` overrides, then `link_*` peers, and finally re-seeds
    `originalData` for dirty tracking. It reads `route.query`, which does not
    change when we stay on the page, so URL-borne pre-fills re-apply for free.
    The reset is otherwise **clean** — see the carry-over decision below.
  - `validateFormField()` (`internal/dataentryconfig/validate.go:591-627`) is
    the template for the new config key's validation: `clear_when_hidden` is
    allowlist-checked so "a typo must not silently resolve to a destructive
    default", *and* has a coherence check rejecting the key where it "could
    never apply" rather than leaving it silently inert. The new key gets the
    same two-part treatment.
  - `FormRelation.Span` (`config.go:553-558`) is the precedent for the opposite
    case: a key captured **only so it can be rejected**, because yaml.v3 would
    otherwise drop an unknown nested key in silence and the author would get
    "no error and no effect".
  - `loadTemplates()` (`:680`) restores template-derived body content.
  - `refreshStagedAffordances()` (`:711`) recomputes affordance-filtered field
    visibility for a staged (not-yet-created) entity; already awaited for the
    initial create paint (`:1668-1670`).
  - `wizard.goTo(0)` (`useFormWizard.ts:285`) resets the step and syncs `?step=`.
  - `PendingButton` (`components/common/PendingButton.vue`) gives the label
    swap + repeat-click suppression the primary Create already uses.
  - The onMounted create branch (`:1652-1670`) is the exact sequence to reuse —
    the reset function should be extracted from it so the two cannot drift.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Make the create path's *terminal outcome* a parameter, and extract the blank-form
setup that `onMounted` already performs so the reset cannot drift from it.

0. **New per-field config key: `keep_on_add_another`.** The reset is clean by
   default; a field opts *out* of being cleared. Named for the button so its
   meaning is unambiguous at the point an author reads it, and so it cannot be
   mistaken for a general-purpose stickiness that applies to ordinary page
   loads (it does not — it fires only on this action).

   ```yaml
   forms:
     new-task:
       entity_type: task
       fields:
         - property: project
           keep_on_add_another: true   # batch context — survives
         - property: title             # cleared, like everything unmarked
       relations:
         - relation: assigned-to
           keep_on_add_another: true
   ```

   - **Go:** `KeepOnAddAnother bool` with
     `yaml:"keep_on_add_another,omitempty" json:"keep_on_add_another,omitempty"`
     on **both** `FormField` (`config.go:322`) and `FormRelation` (`:540`) —
     relations are exactly the batch context most worth keeping (`project`,
     `assigned-to`), so omitting them would gut the feature. Nested keys are
     dropped silently by the decoder (`config.go:814`), so the struct tag is
     what makes the key exist at all; `FormRelation.Span` is the standing
     reminder of what happens without it — **but copy only its yaml lesson**
     (RR-ZK28ZP). `Span` is tagged `json:"-"` (`config.go:560`): captured for
     validation, deliberately *not* serialized. `keep_on_add_another` must
     serialize or the SPA never sees it.

     Note also (RR-ZK28ZP) that `checkUnknownKeys` (`validate.go:360`) walks
     only **top-level** keys and the unmarshal is non-strict, so a typo'd
     `keep_on_add_other:` yields no error and no effect. The struct tag is not
     a complete mitigation; this key inherits that known gap (TKT-QHF4JQ) and
     the docs entry should be the discoverability backstop.
   - **Validation** in `validateFormField` / `validateFormRelation`. `bool`
     needs no allowlist (yaml rejects a non-bool), so only a coherence check is
     in question — and **the obvious one does not hold.** An earlier draft
     proposed rejecting the key on a form with `mode: edit`; checking the code,
     `Mode` has no Go consumer at all and its only frontend use is
     `getEditFormId` (`frontend/src/types/config.ts:263-279`), which treats
     `mode: edit` as *"prefer this form when editing"* — with an explicit
     fallback to any form for the type. A `mode: edit` form is therefore still
     reachable as a create form, so rejecting the key there would refuse a
     legitimate config. **Dropped.**

     What remains defensible is the inert-key check with a real basis:
     `keep_on_add_another` on a `hidden: true` field, whose value the user never
     enters, so carrying it over can express nothing the field's `default`
     doesn't already say. That is the true analogue of "sets clear_when_hidden
     but nothing can hide it", and it **is** well-founded: `hidden` fields are
     filtered out of rendering (`FormFieldList.vue:62`,
     `v-if="field.property && !field.hidden"`) and out of the property set at
     `DynamicForm.vue:828` (`!!f.property && !f.hidden`), so the user cannot
     enter a value there for the reset to carry.

     General principle for this key, learned from the `mode` misstep above: a
     wrong rejection breaks a working operator config, which is worse than a
     silently inert key. Ship a coherence check only where the inertness is
     provable from the code, as it is here — not where it merely seems likely.
   - **Transport:** no API change — `Form` is already serialized to the SPA
     wholesale, so the new JSON field arrives once the tag exists.
   - **TS:** add `keep_on_add_another?: boolean` to `FormField`, `FormRelation`
     and `FormFieldOrRelation` in `frontend/src/types/config.ts`.

   *Default `false` (clean reset) is the safe direction:* a field wrongly
   cleared costs the user one re-entry, while a field wrongly carried over
   silently writes stale data into every subsequent record — the failure this
   ticket must not introduce.

1. **Extract `resetCreateForm()`**, generalizing the partial reset
   `selectTemplate()` (`:792-803`) already performs and the sequence the
   `onMounted` create branch (`:1652-1670`) inlines. `onMounted` and
   `selectTemplate` then both route through it, so one definition of "a fresh
   create form" serves all three callers and they cannot drift.

   **Structure it so the "did we miss a field?" question is answerable by
   reading a type, not by auditing 2459 lines.** Extract a pure
   `blankCreateState(config, entityType, query, keep)` returning
   `{ formData, relations, content }` as plain data; `onMounted`, the reset and
   `selectTemplate` all assign from it. That makes the carry-over logic
   unit-testable with no mount, no router and no fetch (AC-4b), and turns the
   completeness argument from discipline into a return type. The imperative
   parts that cannot be pure — `hiddenPolicy.releaseAll()`, the
   `formGeneration` / `saveGeneration` bumps, affordance refs, the latch —
   stay in `resetCreateForm()` around it.

   It takes the carry-over set from step 0 — `keepProps` / `keepRels`, computed
   from `allFields.value` (not the affordance-filtered `fields`, matching what
   `initializeDefaults` does at `:633` and the reason RR-00VT made that explicit).
   Values for those keys are captured **before** the clear and re-applied
   **after** `initializeDefaults()`, so a kept value wins over the default it
   would otherwise be reset to. Everything else is cleared.

   It must clear/re-seed **all** create-mode state. The list below was
   **rewritten after design review (RR-ZYU2GC)** — the first draft missed six
   refs, several of which cause silent wrong-data writes. Enumerated from the
   component:

   - values: `formData`, `relations` (minus the carry-over set), `content`
     (never kept — a body is per-record by nature and there is no field-level
     key to mark it)
   - validation/interaction: `errors`, `userTouched`. **`userTouched` needs
     care**: `visibleWritablePropertiesForCommit()` (`:437-458`) uses it as the
     RR-2U2D preserve rule, so a carried-over value re-applied as *untouched*
     survives the commit filter only if `stagedVisibleProps` happens to contain
     it. Re-add kept props to `userTouched` after re-applying them.
   - staged affordances: `stagedVisibleProps` (`:186`), `stagedAffordancesReady`
     (`:184`), `fieldAffordances` (`:107`), `relationAffordances` (`:116`).
     Leaving these retains record one's dry-run verdicts — and
     `refreshStagedAffordances` fails *open* (`:797-802`), so on failure the
     stale verdicts persist with no signal. Reset `stagedAffordancesReady` to
     `false` and `await` the new dry-run before the form is interactive,
     accepting the same brief unfiltered state the mount path already has
     (the F19 note at `:908-912`); do not paper over it by keeping record one's
     verdicts.
   - hidden-field policy: `hiddenPolicy.releaseAll()`. `loadEntity` calls this
     (`:499`) precisely because "retained values belong to the form state we
     are about to replace". A reset replaces form state wholesale, so the same
     applies — otherwise a value retained while hidden on record one is
     restored over a cleared field when revealed on record two.
   - `formGeneration` (`:1308`) — bump it. Its own comment says it is "bumped
     whenever the form's underlying entity state is replaced wholesale", which
     is exactly this; without it a `clear_when_hidden: confirm` dialog opened
     before the submit resolves against the *new* record.
   - `linkParams` (`:102`) — idempotent (same query re-parsed), but note the
     `as === 'from'` auto-link `createRelation` (`:1146-1158`) re-runs per
     record. That is correct (each new entity links to the peer) and AC-2 must
     say so, since it is a second API call beyond the create.
   - widget remount: bump `saveGeneration` — **load-bearing, see step 1a**.
   - id controls: `idControls.reset()` (already called by `initializeDefaults`)
   - the one-shot latch: `createdEntityId` (step 3)
   - dirty tracking: `dirty` — but see step 6 for the `adoptLockedFieldValues`
     hole; `initializeDefaults()`'s re-seed of `originalData` is not sufficient
     on its own.

   `stagedFiles` and `dirty` are **already** cleared at `:1177-1178` on the
   existing path; the reset must not assume it is introducing that.

   Not in the list, and deliberately: `pendingCardChanges` (`:1436`) is always
   empty on a create path (`handleSubmit` says so at `:1128-1130`), so clearing
   it is a no-op kept only for symmetry.

1a. **`saveGeneration` is currently dead code, and this ticket makes it
   load-bearing (RR-ICL8CW, RR-VVLNSU).** It is declared at `:227` and bound at
   `:1963`, but `grep -rn saveGeneration frontend/src/` shows it is **never
   incremented**. That matters because it is the `:key` of both relation
   widgets (`FormFieldList.vue:81`, `:93`), and the incoming-picker case
   *requires* a remount:

   an incoming `RelationPicker` keeps its selection in its own `incomingValue`
   ref (`RelationPicker.vue:88`) and `effectiveValue` (`:134`) ignores
   `props.value` entirely, so clearing the parent's `relations` does nothing to
   it. Its record-one peers then re-emit as pure additions against an
   `incomingOriginal` of `[]` (`:184-190`) and get written to record two as
   duplicate links. Bumping `saveGeneration` remounts the picker, which is the
   fix. Treat the bump as required, not belt-and-braces, and pin it with the
   incoming-picker AC (4d).

   Note `RelationCards` renders only when `entityId` is set
   (`FormFieldList.vue:80`), so it never appears on a create form — the widget
   actually under test here is `RelationPicker`.

   then re-run `initializeDefaults()`, re-apply the template, **then** re-apply
   the kept values, then `wizard.goTo(0)` and `await refreshStagedAffordances()`.

   **Do NOT call `loadTemplates()` (RR-RV7WHL).** It re-fetches over HTTP on
   every record, and it unconditionally sets `selectedTemplate = templates[0]`
   and applies it (`:680-693`) — discarding the user's chosen template pill.
   `templates` is never invalidated during the session, so the refetch buys
   nothing. Call
   `applyTemplate(templates.value.find(t => t.name === selectedTemplate.value))`
   against the already-loaded ref instead.

   **Ordering is load-bearing:** `applyTemplate` writes `formData` (`:766-791`),
   so kept values must be re-applied *after* it or a template naming a
   `keep_on_add_another` property clobbers the carried-over value — the exact
   inverse of AC-4b. The final order is: capture kept → clear → 
   `initializeDefaults()` → `applyTemplate(chosen)` → **re-apply kept** →
   `wizard.goTo(0)` → `await refreshStagedAffordances()`.

2. **Thread the outcome through `handleSubmit`.** Give it a parameter —
   `handleSubmit(mode: 'navigate' | 'again' = 'navigate')` — used only at the
   two terminal points:
   - the `createdEntityId` latch (`:1040`), and
   - the final navigation block (`:1210-1220`).

   Everything between (validation, payload assembly, `createEntity`, auto-link,
   staged uploads, the upload-failure branch) is shared verbatim. This is the
   whole point: the two buttons must not be able to build different payloads.

3. **Make the one-shot latch outcome-aware, do not delete it.** `createdEntityId`
   exists because a completed create followed by a second submit mints a
   duplicate (RR-4QO887), and its comment is explicit that navigation is
   asynchronous so the form stays live. Under `mode: 'again'` there is
   deliberately no navigation, so the latch is *more* necessary, not less — but
   it must be released as part of the reset, since a second create is now the
   intended outcome. So: keep setting it at `:1137`, keep the guard at `:1040`,
   and clear it inside `resetCreateForm()`. The ordering is load-bearing — the
   latch is only cleared once the form is genuinely blank again, so the window
   between "created" and "reset" stays protected. `saving` continues to guard
   the in-flight window as before.

   **The reset must be `await`ed INSIDE `handleSubmit`'s `try` (RR-HYYXXK).**
   It is async, so if it were fire-and-forget — or moved after the `finally`
   that sets `saving = false` (`:1242`) — there would be a window with
   `saving === false` and `createdEntityId === null` while the reset is still
   in flight. `handleKeydown` (`:1616-1624`) calls `handleSubmit()` *directly*,
   bypassing `PendingButton`'s repeat-click suppression, so Cmd+Enter in that
   window submits a half-reset form. Awaiting inside the `try` keeps `saving`
   true across the whole reset and closes it.

   Relatedly, the `mode` parameter must be genuinely **defaulted** to
   `'navigate'`, not required: `defineExpose`'s `submit: () => handleSubmit()`
   (`:1839`) is the inline-create host's entry point and calls it bare.

   Guard `stagedUnmounted` after **each** await in the reset, not once
   (RR-QP73I4) — it is a plain `let` (`:707`) and the reset adds two awaits
   past the existing check at `:1207`, reopening the RR-2PZB write-to-dead-refs
   hazard that the checks at `:735`/`:738`/`:799` exist to prevent.

4. **Success toast names the id.** In `'again'` mode replace
   `'Entity created successfully'` (`:1195`) with a message carrying
   `entity.id` — the user is not navigated to the entity, so the id is the only
   evidence it exists. This is a **plain string, not a link**: the `Toast` type
   (`stores/ui.ts:5-10`) has no action/link field and `success()` (`:125`) takes
   a message only, so a clickable toast would mean extending the toast store and
   the `ToastContainer` rendering — a broader change than this ticket, and one
   that would affect every toast in the app. Naming the id satisfies the ticket's
   "reachable somehow" requirement (the user can search or navigate to it); a
   linked toast is a reasonable follow-up ticket, not a prerequisite. The
   upload-failure branch (`:1188-1194`) keeps precedence exactly as it does
   today: it already replaces the success toast rather than adding to it, and
   that behavior is unchanged.

4a. **Surface a swallowed auto-link failure in `'again'` mode (RR-DBEW6R).**
   The `as === 'from'` auto-link `createRelation` (`:1150`) is caught and
   swallowed with a bare `console.warn` (`:1152-1155`). On the navigate path
   the user lands on the entity and can see the missing link; staying on the
   form they never will, so repeated entry can silently produce N unlinked
   entities. Add a toast for this case on the `'again'` path.

4b. **Announce the reset to assistive tech (RR-L44IGF).** The form's only live
   region is the error summary (`role="alert"`, `:1993`, gated on
   `errorCount > 0`), so a screen-reader user gets no signal that the record
   was created and the form cleared. Add a `role="status"` region announcing
   the created id and the reset. This workflow benefits keyboard/AT users more
   than mouse users, so silence here is the wrong default.

5. **Reset only on success.** `resetCreateForm()` is called at the same point
   the navigate path calls `router.push` — inside the `try`, after uploads,
   after the `stagedUnmounted` re-check (`:1207`). A thrown error therefore
   leaves the form exactly as the primary Create leaves it, with the user's
   input and rendered validation errors intact (AC-8).

6. **Render the button** beside the primary Create (`:2041`), gated on
   `!isEdit && !props.embedded && wizard.isLastStep.value` so it appears exactly
   where the Create button does and never inside a modal-hosted inline create.
   Use `PendingButton` with `class="btn btn-secondary"` (secondary: the primary
   action stays the one that finishes the job) and `@click` calling
   `handleSubmit('again')`. It must be `type="button"`, not `type="submit"` —
   two submit buttons in one `<form>` would make Enter ambiguous, and the
   `@submit.prevent="handleSubmit"` binding (`:1901`) must keep meaning the
   primary Create.

7. **Focus the first field after reset** (`nextTick`, `document.getElementById`),
   mirroring what `focusFirstError()` (`:1006-1026`) already does with
   `field-${prop}` ids. Without this the user has to click back into the form
   for every record, which cancels out most of the saving the button exists for.

   **One `nextTick` is not enough (RR-L44IGF):** the `saveGeneration` /
   `formGeneration` bumps remount `FormFieldList`, and `refreshStagedAffordances`
   resolves later and can change which fields render. Focus after the last
   thing that can remount — i.e. after the awaited dry-run — and treat it as
   best-effort rather than asserting it in a brittle test.

8. **Fix the dirty hole the re-seed does not cover (RR-1LMJSQ).**
   `initializeDefaults()` re-seeds `originalData` (`:672-676`) and
   `applyTemplate` re-seeds it again (`:786-790`), but `adoptLockedFieldValues`
   (`:749`) — reached from `refreshStagedAffordances`, which runs *after* both —
   mutates `formData` in place without touching `originalData` or calling
   `checkDirty()`. On a form with an entry-locked field, record two is dirty the
   instant the dry-run resolves, with nothing the user did: a spurious
   "unsaved changes" prompt after **every** record. Re-seed `originalData` (or
   re-run `checkDirty()`) after the awaited `refreshStagedAffordances()`, as the
   last step of the reset.

**Alternatives considered:**

- *A checkbox "keep adding" next to Create, changing what Create does.* Rejected:
  the same button then has two behaviors depending on hidden state, and the
  chosen mode persists invisibly across submits. An explicit second button
  makes each click's outcome unambiguous, which is also what Django/Salesforce
  settled on.
- *Navigate to a fresh create route (`router.push` to the same path).* Rejected:
  the route is unchanged so Vue Router will not remount the component, and
  forcing it with a `:key` would tear down and rebuild the form for no benefit —
  losing the in-memory `formConfig`/template state we just loaded and adding a
  loading flash between every record.
- *Reset to "whatever the user just typed" (sticky all fields).* Rejected: it
  makes duplicate rows the default outcome, and `unique:` properties would fail
  validation on every second record.
- *Infer stickiness from the URL — carry over whatever arrived as a
  `prop.*`/`rel.*`/`link_*` pre-fill, clear everything else.* This was the
  earlier plan and was **rejected on review**: it is implicit policy derived
  from how the user happened to arrive at the form, so the same form carries
  different fields over depending on whether it was opened from a list button,
  a document link, or a bookmark. An operator cannot express "always keep
  `project` when batch-entering tasks" without engineering a URL for it, and a
  user cannot predict the behavior. Replaced by explicit per-field config
  (approach step 0), which makes the decision the operator's, visible in
  `data-entry.yaml`, and identical however the form was reached. URL pre-fills
  still re-apply on reset, but as a consequence of the query string being
  unchanged — not as the stickiness mechanism.
- *Deleting the `createdEntityId` latch.* Rejected — see approach step 3. It
  guards a real duplicate-creation hole that gets wider, not narrower, when the
  form stops navigating away.

**Files to modify:**

- `internal/dataentryconfig/config.go` — `KeepOnAddAnother` on `FormField`
  (`:322`) and `FormRelation` (`:540`).
- `internal/dataentryconfig/validate.go` — coherence check in
  `validateFormField` (`:591`) and `validateFormRelation`.
- `internal/dataentryconfig/validate_test.go` — AC-4c.
- `frontend/src/types/config.ts` — `keep_on_add_another?: boolean` on
  `FormField`, `FormRelation`, `FormFieldOrRelation`.
- `frontend/src/components/forms/DynamicForm.vue` — the whole change: extract
  `resetCreateForm()`, parameterize `handleSubmit`, clear the latch in the
  reset, add the button to the actions row.
- `frontend/src/components/forms/DynamicForm.attachments.test.ts` — despite the
  name this is where the **create-submit path** is already tested end to end
  (its `submit(wrapper)` helper at `:214`, `:341` "a second submit after a
  failure does not create a second entity", `:351` "still navigates to the
  created entity"). AC 2, 3, 5, 8 belong beside those, reusing the helper —
  `:341` in particular is the existing latch regression test that AC-5 must not
  break. If the create-submit cases outgrow the attachments file, split them to
  a new `DynamicForm.create.test.ts` rather than duplicating the harness.
- `frontend/src/components/forms/DynamicForm.test.ts` — AC 1, 4, 6, 9.
- AC-7 goes in a mounting test (beside the create-submit cases), **not**
  `DynamicForm.guard.test.ts` — that file replicates the guard in a tiny
  stand-in component (`makeFormHarness`, `:19-36`) and never mounts
  `DynamicForm`, so extending it would prove nothing (RR-1LMJSQ).
- `frontend/src/components/forms/DynamicForm.embedded.test.ts` — assert the
  button is absent when embedded (AC-1). Its existing "emits the created entity
  instead of navigating" / "still navigates when not embedded" pair is the
  closest analogue to the new stay-on-page outcome and the model to follow.
- `e2e/tests/fixtures.ts` — a dedicated test form in the inline
  `DATA_ENTRY_YAML` (written to `data-entry.yaml` at `:384`) exercising
  `keep_on_add_another`, following the `task_clear_when_hidden` form at
  `:1185-1198` which pins the analogous key. Without this the e2e spec has no
  config to drive the carry-over path.
- `e2e/tests/create-add-another.spec.ts` (new) — the
  two-entities-without-navigation flow, plus carry-over.
- `docs/data-entry.md` — document the action in the forms section.

**Dependencies:** none new. Existing internal APIs only: `PendingButton`,
`useFormWizard.goTo`, `uiStore.success`, `initializeDefaults`, `loadTemplates`,
`refreshStagedAffordances`, `idControls.reset`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

This ticket introduces **no new server surface and no new request input**. It
does add one **operator-authored config input** — `keep_on_add_another` in
`data-entry.yaml`. Per CLAUDE.md the configuration is not a secret and this key
is not confidential; it is trusted operator input, validated at load
(allowlist/coherence, failing the load rather than degrading silently), which is
the same treatment `clear_when_hidden` gets. It cannot widen anyone's access: it
only decides whether a value the user already typed stays in their own form.

The new button calls the same `handleSubmit` body, which calls the same
`createEntity` endpoint with the same payload; validation
(`validate(scope, requiredWhenProps(scope))`, `:1052`) is unchanged and runs
before every submit, including the new one.

- *Form field values* — user input, already validated client-side by the
  existing `validate()` and authoritatively server-side by entitymanager. The
  server remains the enforcement point; the reset changes nothing about it.
- *`prop.*` / `rel.*` / `link_*` query params* — re-read on reset from the
  unchanged `route.query` via the existing `initializeDefaults()`. Same values,
  same parsing code, same page load; re-applying them grants nothing that the
  initial page load did not already grant.
- *`return_to`* — read once on mount through the existing `readReturnTo(route.query)`
  allowlist (`:579`), which is what prevents an open-redirect. The `'again'`
  path never navigates, so it does not consume `returnTo` at all; the reset must
  NOT re-run `applyReturnToFromQuery()`, keeping the sanitized value read at
  mount as the single source.

**Security-Sensitive Operations:**

- *ACL / authorization* — unchanged and entirely server-side. Each create is an
  independent authorized POST; N creates from one form instance are
  indistinguishable to the server from N creates from N page loads. The client
  gains no capability by not navigating.
- *Staged file uploads* — `uploadStagedFiles` runs unchanged, against the id the
  server just minted. The reset clears `stagedFiles` **after** the upload phase,
  reusing the existing ordering that RR-6ZAOMK and RR-4QO887 established;
  attachment validation is server-side and untouched.
- *Duplicate-write hazard* — the genuine risk in this change, and the reason the
  `createdEntityId` latch is preserved rather than removed (approach step 3).
  Covered by AC-5 and AC-8 and by the `saving` re-entrancy guard.
- *Error handling* — the failure path is untouched; errors surface through the
  existing `ApiError` / `validationErrors` handling and the toast. The only new
  message is a success toast containing an entity id the user just created and
  is authorized to see, so it discloses nothing.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Where |
|----|------|-------|
| 1 | Button present in create; absent in edit; absent when `embedded` | `DynamicForm.test.ts`, `DynamicForm.embedded.test.ts` |
| 2 | `createEntity` called with payload identical to primary-Create case | `DynamicForm.attachments.test.ts` (`submit` helper) |
| 3 | `router.push` not called after the action | `DynamicForm.attachments.test.ts` (mirrors `:351`) |
| 4 | Filled field returns to default; `?prop.x=y` pre-fill re-applied; template content re-applied | `DynamicForm.test.ts`; `blankCreateState` unit test |
| 4d | Incoming `RelationPicker` selection does not reach record 2's payload | `DynamicForm.test.ts` |
| 4b | Marked field + marked relation carry over, unmarked field does not; kept value beats the default | `DynamicForm.test.ts` |
| 4c | Key round-trips on both `FormField` and `FormRelation` (asserting `true`, not merely no error); `hidden`-field rejection if that check ships | `internal/dataentryconfig/{config,validate}_test.go` |
| 5 | Two sequential creates from one mounted form → `createEntity` twice, different titles; existing `:341` latch test still passes | `DynamicForm.attachments.test.ts` |
| 6 | `uiStore.success` message contains the created id | `DynamicForm.test.ts` |
| 7 | Not dirty after the action (incl. after an entry-locked field's dry-run resolves) | A test that MOUNTS the component and reads `defineExpose`'s `isDirty()` — **not** `DynamicForm.guard.test.ts`, which uses a hand-copied replica harness (RR-1LMJSQ) |
| 8 | Rejecting `createEntity` → form values intact, no success toast, no reset | `DynamicForm.attachments.test.ts` |
| 9 | 2-step wizard returns to step 0, `?step=` follows, and the leave-guard does NOT fire on that `router.replace` (RR-P8SANC) | `DynamicForm.test.ts` |

**Integration (e2e, `e2e/tests/`):** the flow no unit test covers — real router,
real server, real create. Navigate to a create form, fill and use
"Create & add another", assert the URL is still the create form and the fields
are blank, fill a *different* title and use the primary Create, then assert
**both** entities exist in the list view. This is the test that would catch a
payload that silently carried the first record's values into the second.

**Edge Cases:**

- *Empty form + action* → validation fails, `focusFirstError()` runs, nothing is
  created, no reset. (Same as primary Create.)
- *Second record identical to the first, on a type with a `unique:` property* →
  server rejects; the error surfaces and the form keeps the input. Explicitly
  NOT special-cased — it is the same failure the user would get from two
  separate page loads.
- *Staged attachments* → uploaded against the first entity; `stagedFiles`
  cleared by the reset so record two starts with no files. A record-one upload
  failure shows the error toast and (per AC-8 ordering) must not be silently
  swallowed by the reset.
- *Multi-step wizard mid-step* → the action only renders on the last step, so it
  cannot fire from step 1; after success, back to step 0 (AC-9).
- *Component unmounted mid-upload* (`stagedUnmounted`) → the existing re-check
  at `:1207` guards the router push; the reset must sit behind the same check so
  it does not touch a torn-down component.
- *Rapid double-click / `Cmd+Enter` during the action* → `saving` guard plus
  `PendingButton`'s repeat-click suppression, then the `createdEntityId` latch
  until the reset completes.
- *A form whose only fields are defaults (no user input)* → creates a second
  entity identical to the first. Correct: the user asked for it twice.

**Negative Tests:**

- `createEntity` rejects with a network error → no toast claiming success, no
  reset, no navigation, form still dirty.
- `createEntity` rejects with `ApiError` carrying `validationErrors` → per-field
  errors render exactly as for the primary Create; no reset.
- Client-side validation fails → no POST at all; first errored field focused.
- Attempted use in edit mode (button forced into the DOM in a test) →
  `handleSubmit` early-returns on `isEdit` (`:1050`); no write. This asserts the
  edit-mode exclusion is enforced in the handler, not merely by hiding the
  button.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Severity | Mitigation |
|------|----------|------------|
| Incomplete reset leaks record one's state into record two (a wrong-data bug, silently) | **High** | **Design review found six missed refs on the first draft (RR-ZYU2GC) and a seventh path via incoming pickers (RR-VVLNSU) — this risk was real, not theoretical.** Mitigation is now structural, not diligence: a pure `blankCreateState()` makes the value half a return type, and the imperative half is enumerated in step 1 against the component. AC-4, 4b, 4d, 5 plus the e2e two-entity assertion target it. |
| Weakening the `createdEntityId` latch reopens the duplicate-create hole RR-4QO887 closed | **High** | Latch kept and cleared only inside the reset, once the form is blank; AC-5 is its regression test. Documented in the code comment at the clear site with the RR reference. |
| `handleSubmit` grows a second code path and the two buttons diverge | Medium | Only the two terminal points branch on `mode`; payload assembly stays single-path. Reviewable as a small diff. |
| Dirty-guard false positive after reset (spurious "unsaved changes" prompt) | Medium | `initializeDefaults()` re-seeds `originalData`, which is what `dirty` compares against; AC-7 covers it. |
| A `keep_on_add_another` field silently writes stale data into every later record | **High** | The reason the default is `false` / clean. A carried-over value is only ever one the operator explicitly asked to carry; AC-4b pins both directions (kept *and* not-kept) so a broadened carry-over set fails a test. |
| The new config key is dropped in silence by yaml.v3 if the struct tag is missed | Medium | Exactly the `FormRelation.Span` failure mode, already documented in-tree; AC-4c's load-error test only passes if the key is actually decoded. |
| Config key added to Go but not to the TS types, so the SPA ignores it | Medium | AC-4b is a frontend test driven from a form config carrying the key — it fails if the type/plumbing is missing. |
| A child widget holding its own state survives the reset (the incoming-picker class of bug) | **High** | `saveGeneration` bump remounts both relation widgets; AC-4d pins the incoming case. When adding any future create-form widget, check whether it holds state outside `formData`/`relations`. |
| Spurious "unsaved changes" prompt after every record | Medium | Step 8 re-seeds `originalData` after the awaited dry-run; AC-7 covers it on a form with an entry-locked field. |
| Users mistake the secondary button for the primary and never leave the form | Low | Primary/secondary styling; "Create" stays `btn-primary` and keeps the `Cmd+Enter` binding. |
| Two submit-type buttons make Enter ambiguous | Low | New button is `type="button"` with an explicit `@click`. |

**Effort:** `l` — raised from `m` on review feedback. The button and reset alone
were `m`; adding an operator-facing config key takes it across a package
boundary (`internal/dataentryconfig` struct + validation + tests, the TS types,
the docs reference entry) and adds three acceptance criteria. A new key in
`data-entry.yaml` is also a permanent API surface: it must be named and
validated well the first time, because removing it later breaks operator configs.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — **two** additions. (1) The "Create & add another"
      action in the forms section: the reset is clean, and the created entity is
      named in a toast. (2) A `keep_on_add_another` reference entry alongside the
      existing `clear_when_hidden` one (`docs/data-entry.md:849`), with the
      task/project YAML example — an operator-facing config key is undiscoverable
      unless it is in the field reference, and doubly so given that a typo in it
      is silently ignored (RR-ZK28ZP). **But not *inside* that section:**
      `clear_when_hidden` sits under wizard conditions, and
      `keep_on_add_another` is not a condition key — put it wherever form-field
      keys are enumerated, or it will be read as condition-scoped.
      (3) State explicitly what happens to the **body** (never kept), since an
      operator will ask.
- [x] ~~`docs/metamodel.md`~~ (N/A: `data-entry.yaml` config, not schema.yaml)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] ~~`CLAUDE.md`~~ (N/A: no new pattern or convention)
- [x] ~~`README.md`~~ (N/A: not a project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 12 review-responses, all addressed above. Verdict
was **"do not implement as written"** — the revision below is what makes it
implementable. Every finding was verified against source before acceptance.

*Critical:*

- **RR-ZYU2GC** — reset state enumeration missed six refs
  (`stagedVisibleProps`, `stagedAffordancesReady`, `fieldAffordances`/
  `relationAffordances`, `hiddenPolicy`'s retained map, `formGeneration`,
  `linkParams`). → step 1 rewritten; `blankCreateState` extraction added.
- **RR-VVLNSU** — an incoming `RelationPicker` keeps its selection through the
  reset and re-writes it as duplicate links. → step 1a; AC-4d added.
- **RR-ICL8CW** — `saveGeneration` is never incremented (dead code the plan was
  treating as a proven lever); `RelationCards` never renders in create mode, so
  AC-4's original assertion was untestable. → step 1a; AC-4 corrected.

*Significant:*

- **RR-ZK28ZP** — `FormRelation.Span` is `json:"-"`; the new key must
  serialize. Typo'd keys are also undiagnosable (top-level-only strict check).
  → step 0 corrected.
- **RR-RV7WHL** — `loadTemplates()` would clobber kept values, discard the
  user's template pill, and refetch per record. → step 1 ordering rewritten;
  `applyTemplate` against the loaded ref instead.
- **RR-1LMJSQ** — AC-7 was assigned to a test file that never mounts the
  component; `adoptLockedFieldValues` leaves record two spuriously dirty.
  → AC-7 relocated; step 8 added.
- **RR-HYYXXK** — the double-submit window stays open unless the reset is
  awaited inside the `try` (Cmd+Enter bypasses `PendingButton`). → step 3.
- **RR-DBEW6R** — a swallowed auto-link failure is invisible on this path.
  → step 4a.

*Minor:* **RR-P8SANC** (guard-vs-`router.replace` ordering → AC-9),
**RR-7J1IHR** (AC-2/AC-3 not assertable as written → both reworded),
**RR-L44IGF** (no AT announcement; focus vs remount → steps 4b, 7),
**RR-QP73I4** (`stagedUnmounted` re-check per await → step 3).
