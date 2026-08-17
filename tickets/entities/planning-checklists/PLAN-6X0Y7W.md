---
id: PLAN-6X0Y7W
type: planning-checklist
title: 'Planning: Form edits: separate propose from commit (policy manager + write queue)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

The defect is structural, not a missing feature. `updateField`
(`DynamicForm.vue:1107`) writes `formData` on its first line and arms the
autosave debounce on its last, with no point in between where a change exists
but is not yet committed. Anything needing to intervene — a confirmation above
all — must therefore run *after* the change is applied and possibly already
sent, then reconstruct the prior state from `lastEdit`, autosave's `pending`,
and `retained`. BUG-FB0LN8 produced four such reconstructions; each passed its
tests and each then failed in manual use.

**Scope:**

IN:
- A `proposeChange` seam in `DynamicForm` that every property edit routes
through: widget → propose → policy decision → (accept) apply + commit.
- A policy manager owning the accept/reject decision and returning side-effects
(`clear these`, `retain those`) for the form to apply.
- A write queue that merges by property before sending.
- `clear_when_hidden: confirm` re-enabled end to end (frontend behaviour +
backend allowlist + config validation), as the consumer that proves the seam.
- A component-level test harness mounting the real `DynamicForm` in edit mode
and driving a real widget edit through to an asserted PATCH.

OUT:
- Relations, content, and attachments keep their current handlers
(`updateRelation`, `updateContent`, `onAttachmentChanged`). They have no
`visible_when` policy and no confirm consumer; widening the seam to them is
unjustified churn. The queue changes underneath them, so their tests are
regression surface, but their call sites do not move.
- The create path. `handleSubmit` early-returns for edit
(`DynamicForm.vue:912`); `pruneWizardHidden` is create-only. RR-O4SRG's
drop-on-commit stands.
- The deep-chain staleness limitation accepted in BUG-FB0LN8 (a dependent field
may stay visible holding a stale value when a grandparent branch hides).
Unchanged here; still documented as a known limitation.
- `SectionEditForm`, which shares `isClearedForType` but has its own edit path.

**Acceptance Criteria:**

1. **A rejected proposal touches nothing.** Given `clear_when_hidden: confirm`
on a field holding a stored value, when the user changes the trigger field so
that field would hide and declines the dialog, then: no write is sent, the
trigger field shows its prior value, and the hidden field's stored value is
intact after reload. *Test:* component-level, mounted `DynamicForm` in edit
mode. Spy **`entitiesStore.update`** — not `fetch` (RR-270F1Z): writes go
through the Pinia store (`useAutoSave.ts:318`) and the existing edit-mode test
already mocks at that layer. Assert zero calls, **and** that `formData[trigger]`
equals the pre-change value, **and** that `hiddenPolicy.retained` is unchanged —
the retain at `applyHidePolicy:305` happens *before* the policy check, so a
decline that leaves a stale entry makes the next reveal restore a value from a
branch the user backed out of.

2. **An accepted proposal commits exactly once, atomically.** Same setup, user
accepts: exactly one request carries both the trigger property and the
`properties_unset` for the cleared field. *Test:* assert a single
`entitiesStore.update` call and inspect its body for both keys. See AC4 — this
is why merging is non-deferrable.

2b. **The debounce cannot outrun the dialog.** With a dialog open, advancing
fake timers well past the debounce window (`vi.useFakeTimers()`, +2000ms against
an 800ms debounce) emits **zero** writes. *Test:* component-level. This is the
"thinking too long committed the change" failure from TKT-7S5735 and the
highest-value single assertion in this plan; the first draft omitted it entirely
(RR-270F1Z).

2c. **Decline, then accept, emits exactly one correct write.** Change the
trigger and decline; change it again and accept. Assert one request with the
correct body. This is the `lastEdit`-consumed-twice failure class.

2d. **A second identical transition re-prompts.** Decline, then repeat the same
trigger change: the dialog must appear again, not be silently accepted or
silently skipped (RR-2S0333).

2e. **The form is not dirty after a decline.** `checkDirty()` must run on the
reject path (RR-81E7VH), or the user gets an unsaved-changes prompt for a change
they explicitly declined.

3. **Set-then-unset of one property within a window resolves.** When a property
is set and then unset before the debounce elapses, the emitted PATCH names that
property in `properties_unset` only — never in both `properties` and
`properties_unset`. *Test:* unit, against the queue.

4. **Distinct properties merge into one PATCH.** Two properties edited within
one debounce window produce one request, not two. *Test:* unit; this is new
behaviour — see Risks. **Non-deferrable** (RR-RTZG5T): AC2 depends on it.
Without merging, an accepted confirm emits two PATCHes, and between them the
entity sits in a state the user never approved — trigger changed, dependent
field still populated. If the second fails (network drop, 422, tab close) that
half-applied state is what persists, which is the outcome this seam exists to
prevent. Atomicity of an accepted proposal is a correctness property, not a
queue optimization.

5. **Policy sees accepted-but-unwritten state as committed.** A second proposal
arriving while a first is still queued is evaluated against the first's accepted
value, not against the last server snapshot. *Test:* unit — propose A (accepted,
queued), propose B whose predicate reads A's property; assert the predicate saw
A's new value.

6. **No regression in autosave.** All 28 `useAutoSave` tests pass unmodified,
except where a test asserts per-property request *counts* that AC4 deliberately
changes — those are updated with the change noted in the PR.

7. **`clear_when_hidden: confirm` validates.** The backend accepts `confirm` in
the allowlist; a config using it round-trips through `validateFormField` without
error, and the existing rejection test is inverted.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the design decisions were settled with the user during
BUG-FB0LN8 and are recorded verbatim on TKT-7S5735 ("Design decisions (agreed
with the user)"). A `/research` doc would restate them. The open questions were
*mechanical* (what exactly does the queue do today), answered by the codebase
survey below rather than by surveying external options.

**Existing Solutions:**

Libraries considered and rejected:
- **A form library with built-in propose/commit (FormKit, VeeValidate,
TanStack Form).** Rejected: `DynamicForm` is schema-driven from the rela
metamodel at runtime, not from a static schema declared at build time, and it
owns wizard steps, ACL affordances, relations, attachments and autosave. The
adoption cost is a rewrite of the whole component, and none of them model
"server-committed vs locally-accepted" — which is the actual requirement.
- **A state-machine library (XState) for the proposal lifecycle.** Rejected as
disproportionate: the lifecycle is propose → accept|reject → queued → sent, four
states with no concurrency between them beyond what the FIFO queue already
serializes.

Reusable code found in-tree (survey of the current edit path):
- `useAutoSave.cancelPendingField` (`useAutoSave.ts:549-562`) — drops a staged
write *without* touching form state. Added and tested during BUG-FB0LN8,
currently called only by its own tests. Its in-code comment names this refactor
as the intended consumer. This is the reject path's primitive.
- `useAutoSave.revertField` (`:564-583`) — the wrong primitive here, and
documented as such: it restores from the `lastSeenServer` baseline, which can
rewind an unrelated already-accepted edit (RR-VFQKCY). The proposal object
carries the exact pre-change value instead.
- `queueTail` (`:182`) — `queueTail = queueTail.then(runPatch, runPatch)`. A
FIFO chain that already guarantees one in-flight PATCH and schedule-order
delivery, chaining on both fulfilment and rejection. The write queue formalizes
merging *on top of* this; the serialization itself is done.
- `attachRelations` (`:438-449`) — the existing precedent for coalescing: a
dirty relations body rides along on the next property/content patch rather than
firing its own request. The property-merge behaviour AC4 asks for is the same
idea generalized.
- `useHiddenFieldPolicy` (106 lines) — already the right shape: it owns
`retained` deliberately outside `formData`, and `policyFor` is already an
injected decision function. The manager extends this rather than replacing it.
- `isClearedForType` (`utils/formValue.ts`) — shared with `SectionEditForm` so
both route clears identically; the manager must call it, not reimplement it.

**Correction to the ticket's premise.** TKT-7S5735 decision #2 says "the queue
merges, it does not concatenate," phrased as a refinement. The survey shows
there is **no cross-property merging today**: each property has its own debounce
timer and `fireProperty` (`:289`) builds a patch for that one property, so N
dirty properties produce N PATCHes serialized through `queueTail`. The only
existing coalescing is relations riding along on a property patch. AC4 is
therefore *new behaviour*, not a tightening — which is why it carries its own
risk entry below.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Three roles, one job each, per the ticket:

```text
Widget ──proposes──► Policy Manager ──commits──► Write Queue ──PATCH──► server
```

> **Revised after design review** (RR-DRRC66, RR-C5DBEB, RR-YWIN6T). The first
> draft asserted this seam worked without tracing *how visibility is computed*.
> It is computed from `formData` (`DynamicForm.vue:212` supplies
> `form: formData.value` to the wizard; `evalCond` reads it via
> `getBindings()`). So "which fields would hide" was only answerable by
> mutating first — the current defect relocated, not fixed. Steps 1–3 below are
> the correction; **hypothetical evaluation is the centrepiece of this design,
> not an implementation detail.**

1. **Make the hypothetical evaluable without mutating anything.** Thread
explicit bindings through the wizard's evaluator: `evalCond` currently calls the
`getBindings` closure, and `activeKeys` is already parameterised by a picker
(`useFormWizard.ts:162`), so adding a bindings parameter is a small, natural
extension rather than a new mechanism. That yields two pure, synchronous
functions:

   ```ts
   type Proposal = { property: string; value: unknown; previous: unknown }
   proposedBindings(p: Proposal): Bindings   // formData + p, no mutation
   wouldHide(p: Proposal): string[]          // activeKeys(current) \ activeKeys(proposed)
   ```

These are unit-testable with **no component mount and no dialog** — which
matters, because this is precisely where the four BUG-FB0LN8 bugs lived. Note
this is *bounded* hypothetical evaluation of one proposal, not the general
"hypothetical-evaluation API" BUG-FB0LN8 ruled out: direct hides only, no
transitive closure.

2. **`proposeChange(property, value)` decides before writing.** It builds the
proposal, calls `wouldHide`, consults the policy, and only then applies. The
widget → `FieldRenderer` → `FormFieldList` → `update-field` emit chain is
untouched; `updateField` becomes the thin wrapper behind `@update-field`. It
returns a discriminated union rather than void:

   ```ts
   { status: 'applied' } | { status: 'rejected' } | { status: 'superseded' }
   ```

`'superseded'` is load-bearing: it is what a `loadEntity(true)` landing
mid-dialog or a wizard step change must produce, and naming it forces every call
site to handle it instead of discovering it as a bug.

3. **Hide-detection moves OUT of the `activeProperties` watcher** (RR-C5DBEB).
The watcher is post-flush — it fires *because* `formData` already changed — so
it can observe a hide but can never gate one. It keeps only its non-destructive
duties: error-clearing (RR-U9ERK) and reveal-restore. It **loses the destructive
`scheduleUnset` effect entirely** (`applyHidePolicy:311`), which moves into
`proposeChange`. *Trace the ordering dependency between reveal-restore and the
removed clear before deleting it* — nobody has, and they currently run in one
callback.

4. **One generation counter, fenced at the proposal** — not three per-hazard
guards. A monotonic `proposalGeneration`, bumped by `loadEntity`, entity switch,
and each new proposal; a dialog result is discarded unless the generation still
matches, yielding `'superseded'`. This subsumes the re-prompt guard (RR-2S0333),
the load-race fence, and step navigation.

5. **Resolve the `useConfirm` singleton collision** (RR-YWIN6T). `useConfirm`
returns its in-flight promise to a concurrent caller (`useConfirm.ts:118`), and
`onBeforeRouteLeave` already calls `confirm()` on a failed commit — so an open
clear-dialog would hand its answer to the navigation guard. Fix: the navigation
guard must refuse to run while a proposal is undecided (and `commitImmediately`
must not flush an unaccepted proposal). If that proves awkward, the
clear-confirm gets its own dialog primitive instead of the singleton. Decide
this **before** implementation, not during.

6. **Optimistic apply (decision #1).** On accept, the form writes `formData`,
calls `checkDirty()`, and enqueues. On reject it restores from the proposal's
own `previous` field — scoped to one decision, never a shared mutable slot (the
`lastEdit`-consumed-twice fix) — **and calls `checkDirty()` again** (RR-81E7VH),
or the form stays dirty for a change the user declined.

7. **Effective-state view for decision #3.** The policy reads through
`effectiveValue(property)`: queued-but-unsent value if one exists, else
`formData`. This is what makes "the policy always sees a consistent world" true.
Note this is a *read* helper and does **not** substitute for step 1 — the first
draft conflated the two.

8. **Extend the queue with per-property merge.** `pending` is already keyed by
property with an `UNSET` sentinel, so set-then-unset resolution is
last-write-wins on that entry — already true. The new work is emitting one patch
for all ripe properties: `fireProperty(property)` becomes `fireDue()`. Three
things this touches that the first draft wrongly called "unchanged in shape"
(RR-CWRQPQ):
   - **No-op suppression** (`useAutoSave.ts:297`) is per-property; `fireDue`
must apply it per entry *while batching* and handle "all suppressed" by sending
nothing — else it fires an empty `{properties:{}}` PATCH where today it fired
zero requests.
   - **The disappeared-key sweep** (`:521`) now spans the whole merged batch.
   - **`commitImmediately`** iterates `Object.keys(timers)` calling
`fireProperty` per key (`:602`); under `fireDue` the first call drains every
ripe entry, so the loop needs review and `fireDue` must early-return
defensively.

9. **Re-enable `confirm`.** Add it to `ValidClearWhenHidden` (`config.go:199`),
invert the validation test, and implement it as the first policy returning
`rejected` on decline.

**Files to modify:**

- `frontend/src/composables/useFormWizard.ts` — **added after review**
(RR-DRRC66). Bindings-parameterised `evalCond`/`activeKeys`; without this the
seam is decorative.
- `frontend/src/components/forms/DynamicForm.vue` — `updateField` →
`proposeChange`; watcher loses its destructive effect.
- `frontend/src/composables/useChangePolicy.ts` — **new**.
- `frontend/src/composables/useConfirm.ts` — **added after review**
(RR-YWIN6T), if the guard fence proves insufficient.
- `frontend/src/composables/useHiddenFieldPolicy.ts` — `confirm` branch; module
docstring updated (it currently documents `confirm` as unimplementable).
- `frontend/src/composables/useAutoSave.ts` — `fireProperty` → `fireDue`;
`cancelPendingField` gains its first real consumer.
- `internal/dataentryconfig/config.go` — `ClearWhenHiddenConfirm` in the
allowlist.
- `internal/dataentryconfig/validate.go` + `validate_test.go` — invert the
rejection.
- `frontend/src/composables/useAutoSave.test.ts` — update count-based assertions
affected by AC4.
- `frontend/src/components/forms/__tests__/DynamicForm.propose.test.ts` —
**new** component-level harness.
- `docs-project/entities/guides/GUIDE-data-entry.md` — document `confirm`
(**source** file; `docs/data-entry.md` is generated, run `just docs`).

**Regression surface (not modified, but must stay green):**
`SectionEditForm.vue:179` is a **live `revertField` consumer** (RR-VXKBYC). It
is out of scope for the seam but squarely inside the blast radius of the step-8
queue change, since `revertField` manipulates the same `timers`/`pending` maps.

**Alternatives considered:**

- *Keep the write-through and add a pre-commit hook inside `useAutoSave`.*
Rejected: the dialog would resolve after `formData` was already mutated, which
is the current defect. It also puts UI concerns inside a transport composable.
- *Make the policy own `formData`.* Rejected by the ticket's stated boundary —
a second god-object.
- *Fix only `confirm` without the seam.* Rejected: BUG-FB0LN8 tried four
variants of exactly this.
- *A form library with propose/commit built in (FormKit, TanStack Form).*
Rejected — see Research; none model "locally-accepted vs server-committed",
which is the actual requirement.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `clear_when_hidden` — operator-authored `data-entry.yaml`. Validated by
**allowlist** (`ValidClearWhenHidden`), unchanged pattern; `confirm` is added to
the allowlist rather than the check being loosened. Invalid value → config
validation error at load, fail-fast.
- Proposed field values — end-user input. Unchanged trust posture: values are
validated server-side on PATCH exactly as today. The seam changes *when* a value
is sent, never *whether* it is validated.
- `visible_when` / `required_when` predicates — operator-authored, evaluated
client-side by the existing wizard evaluator. Not extended here.

**Security-Sensitive Operations:**

- **Field-level ACL redaction must not be widened.** The manager reads
`formData` and retained values, both of which are already redaction-filtered on
the read path (`_redacted`, DEC-T0XIWQ). The manager must not read from any
un-redacted source, and must not surface a retained value for a property the
principal cannot see. Per root `CLAUDE.md`, field *names* are not confidential —
only values — so naming a field in a confirm dialog is fine; echoing a redacted
*value* into that dialog is not. **This is a UX concern, not a security one**
(RR-MYHDH0), and the distinction is worth stating because the next reader will
otherwise build on a secrecy property that was never real. `fieldByProperty`
(`DynamicForm.vue:223`) is built from `allFields`, not the redaction-filtered
`fields`, so `policyFor` can resolve a `clear_when_hidden` for a redacted
property. Harmless today (`applyHidePolicy` only reads `formData`, where
redacted props are absent); under `confirm` it would prompt about a value the
user cannot inspect.
- **No new write path.** All writes continue through the existing autosave
PATCH to the data-entry API, which enforces ACL server-side. The queue merges
requests; it never constructs a write the user did not propose.
- **The confirm dialog is not a security control.** It prevents accidental data
loss, nothing more. Declining must not be relied on to prevent a write the
server would otherwise reject.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the AC list above — each criterion names its test and
level. Integration is the component-level harness (AC1, AC2), which is the layer
that was missing; AC3–AC5 are unit tests against the queue and policy; AC7 is a
Go table test.

**The coverage gap this must close.** Every BUG-FB0LN8 bug lived in
`DynamicForm`'s orchestration and **none** were caught: `useAutoSave.test.ts`
drives the composable directly, never through a widget; of the four files that
mount `DynamicForm`, one is edit-mode but never simulates an edit, two are
create-mode only (so `useAutoSave` never even constructs), and the guard test
replicates the guard in a stub component rather than mounting the real one — so
the edit-mode `commitImmediately` branch is untested. **No test drives a real
widget edit through to an asserted PATCH.** The new harness must do exactly
that, or this refactor reproduces the same blind spot.

**Edge Cases:**

- Decline, then immediately re-trigger the same transition — must re-prompt, not
silently accept. Guarded by the single `proposalGeneration` fence (Approach step
4), never a synchronously-cleared boolean: the watcher is post-flush and would
clear it before observing the revert (RR-2S0333). Now AC2d.
- **Navigation while a dialog is open** — the `useConfirm` singleton would hand
the clear-dialog's answer to the navigation guard (RR-YWIN6T). Distinct from,
and worse than, the flush concern below.
- Vue coalescing a programmatic revert with a same-flush user edit into one
watcher invocation — the reason suppression had to be per-property before.
- Boolean `false` and empty-array values — `isClearedForType` treats boolean
`false` as a real value; a confirm must fire for it and must not treat it as
"nothing at stake".
- Multiple fields hiding at once — one batched dialog naming each field, never
one dialog per field.
- A hide-clear proposal arriving while a user edit for the same property is
still queued.
- `loadEntity(true)` (attachment change, or a state-machine transition) landing
while a dialog is open — `formData` is wholesale replaced; the proposal's
`previous` value is then stale. Must be fenced by a load generation, and
`hiddenPolicy.releaseAll()` already runs on load.
- Navigation while a proposal is undecided — `commitImmediately` must not flush
an unaccepted proposal.
- Autosave not yet constructed (the window between `loading = false` and the
`onMounted` edit branch finishing) — `updateField` guards with `if
(!autoSave.value) return`; the manager needs the equivalent guard.

**Negative Tests:**

- `clear_when_hidden: bogus` → config validation error naming the field
(existing test, must still pass).
- `clear_when_hidden` without `visible_when` and not on a conditional step →
still an error.
- A PATCH rejected with 422 mid-queue → the failing property surfaces a
`fieldError`, other merged properties in that request are not silently reported
as saved. **This needs explicit attention**: merging N properties into one
request means one 422 now implicates N properties, where today it implicates
one. Covered by an explicit test.
- Declining a confirm when the queue already holds an accepted write for a
*different* property → that write still commits.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Effort:** L (unchanged).

**Risks:**

1. **`useAutoSave` is load-bearing for every edit form, not just conditional
ones.** Its 28 tests encode no-op suppression, unset semantics, in-flight abort,
per-channel debounce, and commit-on-navigate. *Mitigation:* that suite is the
regression gate and must stay green; changes to it are limited to count-based
assertions that AC4 intentionally changes, each called out in the PR description
rather than quietly amended.

2. **AC4 (property merging) is new behaviour, not a tightening** — and it
changes error attribution: one 422 now implicates every property in the merged
request. *Mitigation:* explicit negative test; if attribution proves messy, AC4
is the one criterion that can be deferred without invalidating the seam —
merging is a queue optimization, and decision #3 explicitly says the policy must
not care whether the queue is smart or dumb. **Fallback: ship the seam with a
sequential pump, defer merging to a follow-up.**

3. **`DynamicForm.vue` is 2102 lines and grew during BUG-FB0LN8.** This
refactor must not add to it net. *Mitigation:* new logic lands in composables;
the target is `updateField`'s body shrinking.

4. **Scope creep into relations/content/attachments.** They share the queue but
not the policy. *Mitigation:* explicitly OUT above; the queue change is
transparent to them.

5. **The component-level harness may be slow or flaky.** `DynamicForm.test.ts`
already leaks 15 mounted components into vitest teardown (BUG-2OXEW0).
*Mitigation:* **BUG-2OXEW0 is a hard prerequisite** (RR-2SDLTF), not an "or" — a
harness built on a leaking pattern inherits the flake, gets `.skip`'d, and
silently reopens the very coverage gap this ticket exists to close.

6. **The reveal-restore / hide-clear ordering dependency is untraced.** Both
effects currently run in one watcher callback; step 3 removes one of them.
*Mitigation:* trace and pin the ordering with a test before deleting, rather
than after.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — `clear_when_hidden` gains a third value. **Edit
`docs-project/entities/guides/GUIDE-data-entry.md` and run `just docs`** —
`docs/` is generated, and editing it directly fails the Docs CI check.
- [x] `CLAUDE.md` (root or `internal/dataentry/CLAUDE.md`) — the propose/commit
seam is a convention new form code must follow; worth a short rule so the next
edit path does not reintroduce write-through.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] ~~`README.md`~~ (N/A: no project-level change)

Also update the `useHiddenFieldPolicy` module docstring, which currently
documents `confirm` as unimplementable and points at this ticket.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Ten findings; the three critical ones invalidated the first draft's central
claim and the plan above is the revision, not a patch over it.

| ID | Severity | Finding | Addressed in |
|----|----------|---------|--------------|
| RR-DRRC66 | critical | Visibility is computed from `formData`, so the seam still mutated before deciding | Approach step 1; `useFormWizard.ts` added to Files to Modify |
| RR-C5DBEB | critical | Hide-detection cannot live in the post-flush watcher | Approach step 3 |
| RR-YWIN6T | critical | `useConfirm` singleton shares its in-flight promise with the navigation guard | Approach step 5 |
| RR-RTZG5T | significant | AC2 (atomic PATCH) contradicted the AC4 deferral fallback | AC4 marked non-deferrable; Risk 2 rewritten |
| RR-CWRQPQ | significant | Merging breaks no-op suppression, the disappeared-key sweep, `commitImmediately` | Approach step 8 |
| RR-VXKBYC | significant | `SectionEditForm` is a live `revertField` consumer in the blast radius | Regression surface section |
| RR-81E7VH | significant | `checkDirty` never reconciled with the reject path | Approach step 6; AC2e |
| RR-270F1Z | significant | Harness spied the wrong layer and asserted too weakly | AC1 respec; AC2b–2e added |
| RR-2SDLTF | minor | BUG-2OXEW0 stated as an optional "or" | Risk 5 — now a hard prerequisite |
| RR-MYHDH0 | minor | Confirm may name a redacted field | Security Considerations |

**The finding that mattered.** The first draft correctly diagnosed the root
cause and correctly reported every fact about the current code (the reviewer
verified all eight survey claims). It then failed on the *inference*: it
asserted the seam worked without tracing how visibility is computed. Because
`conditionBindings` feeds `form: formData.value` into the wizard, "which fields
would hide" was only answerable *after* mutating — so the design as first
written would have produced a fifth `confirm` implementation that passed its
tests and failed in manual use, exactly like the previous four. Hypothetical
evaluation is now the centrepiece rather than an unexamined assumption.
