---
id: BUGA-QUI067
type: bug-analysis-checklist
title: 'Analysis: Condition-hidden field is pruned on save and can never be revealed again (silent data loss)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally — verified by code trace rather than a running
instance: the destructive path is unconditional and fully covered by an existing
e2e test that asserts it as intended behavior (`e2e/tests/wizard.spec.ts:231`,
AC5). That test seeds `assignee: "alice"`, toggles the governing `done` checkbox
off **without touching assignee**, and asserts the value is wiped. It is the
bug, encoded as a passing test.
- [x] Minimal reproduction steps documented — see BUG-FB0LN8 body (reporter's
`opportunity` / `inkooproute` case). Reduced form: any edit form with a
`visible_when` field holding a stored value; flip the governing property off,
then back on.
- [x] Environment/conditions noted — **edit mode only** (create is unaffected and
correct). Requires the property channel of autosave to be enabled. Backend
agnostic. No ACL configuration needed — this fires for ordinary, fully-permitted
fields.

## Root Cause

- [x] Immediate cause identified (why1) — the `activeProperties` watcher
(`DynamicForm.vue:205-232`) calls `autoSave.scheduleUnset(prop)` the moment a
`visible_when` turns false, which PATCHes `properties_unset: [prop]` server-side
on a debounce. **No save required**; the user sees a green "Saved" indicator.
`useAutoSave.ts:296` deliberately exempts UNSET from no-op suppression, so it
always fires.
- [x] Contributing factors found (why2-3) — the edit-mode render gate
(`fields` computed `:249-261`, `affordanceVisible()` `:693-701`) requires
`f.property in formData.value`, so the just-deleted key can never render again.
That gate infers "absent ⇒ ACL-hidden", which is unsound: an ACL-hidden field is
**byte-identical on the wire** to a merely-unset one (`affordances.go:905-916`
`stripHiddenProperties` deletes with no tombstone; `computeFieldAffordances`
`:715-719` is sparse and skips hidden fields "by design"). Absence is genuinely
ambiguous.
- [x] Systemic cause explored (why4-5) — edit-mode pruning was never justified on
its own terms. It entered via TKT-ZKGY3 as *"the edit-mode analogue of create's
submit-time hidden-branch prune"* — a symmetry argument. The symmetry fails: on
**create** the pruned value was typed this session into an abandoned branch
(RR-O4SRG's actual and only scope — nothing is lost); on **edit** it may be
pre-existing committed data the user never touched. Underneath that, two
independently-reviewed subsystems (the TKT-G7N5 / TKT-3I5U affordance filter,
and TKT-CHLAJ / RR-O4SRG hidden-branch pruning) both key off the same overloaded
signal — presence of a property key — with no test covering their interaction.

## Fix Planning

- [x] Fix approach determined — see the **Agreed fix** section on BUG-FB0LN8.
Five parts: (1) render structure from the metamodel, not from data presence; (2)
retain hidden values client-side; (3) remove the eager `scheduleUnset`; (4) new
`clear_when_hidden: no|yes|confirm` config key, default `no`; (5) `confirm`
gates **before write emission**, and declining reverts the trigger field.

Rejected alternatives, with reasons:
- *Reporter's original* (metamodel render gate alone) — fixes the
dead-end but not the loss; user sees an empty field where their data was.
- *`userTouched` gate alone* — narrows who gets destroyed rather than
stopping destruction; still breaks "user edited, hid, wants it back", and still
loses across a reload.
- *Delete-on-hide / delete-on-navigate-away* — both keep the destructive
write and still need the in-memory copy, so they carry retention's complexity
plus a server delete. Navigate-away additionally drops on crash and produces
`properties_unset` attributed to a principal who never asked for it.

The "leak" premise for eager deletion does not hold: `visible_when` is a
**client-side layout directive** the server never evaluates (RR-O4SRG: *"Server
is not a backstop"*). A hidden field's value is already exposed via `GET`,
search, and export. Hiding never protected it, so deleting on hide closes no
channel — it only destroys data. Genuine value-hiding is `visible:` field-level
ACL, a separate mechanism that works server-side. The legitimate residual
concern is **staleness**, not leakage — which is why clearing becomes opt-in
config rather than hardcoded behavior.

- [x] Regression test planned — `conditional-field-reveal-roundtrip-test`
(linked via `adds-measure`). Pins the **reveal** direction, which no existing
test covers: hide → reveal must restore both the rendered field and its value.
Plus coverage for all three `clear_when_hidden` values and the confirm-decline
revert path. **`e2e/tests/wizard.spec.ts:231` (AC5) must be inverted** — it
currently asserts the destructive behavior. Its replacement asserts an untouched
stored value survives the toggle under the new `no` default, with the old
behavior re-pinned under `clear_when_hidden: yes`.

- [x] Related areas checked for similar issues:
  - `pruneWizardHiddenRelations()` (`DynamicForm.vue:738`) is the relation
analogue and shares the shape. Create-path only in practice, but it must be
checked against the same reveal-direction argument during implementation.
  - The **create** path is correct and stays unchanged — `stagedVisibleProps`
carries visibility explicitly from the dry-run rather than inferring it from key
presence, so it has no ambiguous-absence problem.
  - The RR-U9ERK error-clearing effect shares the same watcher and is
**orthogonal** — it must be preserved when the destructive effect is removed
(hiding an errored step must still clear its flag, else a phantom "N fields need
attention" points at no reachable step).
  - No unit test exists for `pruneWizardHidden` or `affordanceVisible` at
all; all coverage is e2e. That gap is why the interaction went unnoticed.
