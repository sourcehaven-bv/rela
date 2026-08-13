---
id: BUG-FB0LN8
type: bug
title: Condition-hidden field is pruned on save and can never be revealed again (silent data loss)
description: Toggling a visible_when condition off and back on in an edit form permanently destroys the stored values of the conditional fields. pruneWizardHidden() deletes the keys from the saved payload; the render gate then requires the key to be present in formData to show the field again, so the field stays hidden forever and its data is unrecoverable through the UI.
priority: critical
effort: m
why1: 'Toggling a visible_when condition off deletes the conditional field''s stored value (edit mode: the activeProperties watcher calls autoSave.scheduleUnset, sending properties_unset server-side on a debounce; create/explicit-save: pruneWizardHidden drops the key from the payload).'
why2: The field can then never be revealed again, because the edit-mode render gate (fields computed / affordanceVisible) requires the property key to be present in formData — and the unset just removed it. Loss plus no recovery path equals silent permanent loss.
why3: 'The render gate treats ''key absent from properties'' as ''ACL-hidden''. That inference is unsound: the wire shape for an ACL-hidden field is byte-identical to a merely-unset one (affordances.go stripHiddenProperties deletes with no tombstone; computeFieldAffordances is sparse and skips hidden fields by design), so absence is genuinely ambiguous.'
why4: 'Edit-mode pruning was introduced in TKT-ZKGY3 as ''the edit-mode analogue of create''s submit-time hidden-branch prune'' — a symmetry argument. The symmetry does not hold: on create the pruned value was typed this session into an abandoned branch (RR-O4SRG''s actual scope, nothing lost), whereas on edit the pruned value may be pre-existing committed data the user never touched.'
why5: 'Two independently-designed mechanisms — the ACL affordance filter (TKT-G7N5/TKT-3I5U) and wizard hidden-branch pruning (TKT-CHLAJ/RR-O4SRG) — were each reviewed in isolation and both key off the same overloaded signal (presence of a property key), with no test covering their interaction. The reveal direction was never tested: e2e asserts hide-then-unset (AC5) but nothing asserts hide-then-reveal restores. An acceptance criterion codified the destructive behavior as intended before the asymmetry was noticed.'
prevention: 'Pin the hide/reveal round-trip (not just hide) as a regression test. More generally: when a value''s absence is load-bearing for one subsystem (ACL: absence means hidden) do not let another subsystem create that absence for an unrelated reason (hygiene pruning) — an ambiguous sentinel shared by two mechanisms needs an explicit interaction test at design-review time. Prefer narrowing a destructive write to session-touched state over inferring intent from stored state.'
status: done
---

## Reproduction

Config:

```yaml
# metamodel.yaml
opportunity:
  properties:
    inkooproute:          { type: inkooproute }   # enum incl. 'aanbesteding', 'onderhands'
    inschrijfdeadline:    { type: date }
    vragenronde_deadline: { type: date }
```

```yaml
# data-entry.yaml — edit_opportunity
fields:
  - property: inkooproute
  - property: vragenronde_deadline
    visible_when: "form.inkooproute == 'aanbesteding'"
  - property: inschrijfdeadline
    visible_when: "form.inkooproute == 'aanbesteding'"
```

Steps:

1. Entity has `inkooproute: aanbesteding`, `inschrijfdeadline: 2026-09-15`, `vragenronde_deadline: 2026-08-20`.
2. Open the edit form. All three render correctly.
3. Change `inkooproute` to `onderhands`. Both deadline fields hide — correct.
4. Change `inkooproute` back to `aanbesteding`.

**Expected:** both deadline fields reappear with their stored values.

**Actual:** they do not reappear, in this session or any later one. The stored
dates are gone — verified via `GET /api/v1/opportunities/<id>`: both are now
`null`.

## Cause

Two individually-reasonable mechanisms that are mutually destructive.

**1. Prune-on-hide guarantees the key is absent.** `pruneWizardHidden()`
(`frontend/src/components/forms/DynamicForm.vue:722`, used at `:881` and `:907`)
drops condition-hidden keys from the submitted payload, so the values are
deleted on save. In edit mode with autosave the `activeProperties` watcher
(`:205-232`) is even more direct — it calls `autoSave.scheduleUnset(prop)` the
moment the field hides, so the loss lands without any explicit save.

**2. Render-requires-key guarantees it stays hidden.** Both the flat `fields`
computed (`:249-261`) and `affordanceVisible()` (`:693-701`) gate on:

```ts
if (f.property in fieldAffordances.value) return true
if (isEdit.value && f.property in formData.value) return true  // ← must already exist
return false
```

`fieldAffordances` (`_fields`) cannot help: `computeFieldAffordances`
(`internal/dataentry/affordances.go:696`) is **sparse by design** — "only fields
whose verdict deviates from the permissive default appear" — so a
normally-writable field is never listed. The `formData` presence check is the
only gate, and step 1 just removed the key.

The invariant "a key absent from `properties` means ACL-hidden" is false: a key
can also be absent because it was never set, or because we ourselves deleted it.

## Impact

Silent, irreversible loss of user data from a single dropdown toggle, with no
error, no warning, and no UI path to recovery. In the reporter's deployment
these are public-procurement submission deadlines, where a missing date means
automatic exclusion from a tender.

Note this is only unrecoverable *through the UI* — on the postgres backend the
prior values remain in entity version history, and on fsstore in git history.
That is a forensic recovery path, not a user-facing one.

## Suggested fix (from reporter)

In edit mode, decide renderability from the entity type's declared properties in
the metamodel, and reserve the `formData`/`_fields` check for genuine ACL-driven
hiding. Optionally, don't prune a hidden field's *existing* value — only skip
newly-entered values for branches that are not taken.

The frontend already has what the first half needs:
`schemaStore.getEntityType(type).properties` is loaded on app mount.

## Agreed fix

The framing that drove this: **form structure and form data are conflated.** The
form asks two unrelated questions through one lookup — *does this field exist in
the form?* (structure: static, from the metamodel + `data-entry.yaml`) and *what
is its value?* (data: per-entity, per-principal). Because the render gate
answers the structure question by inspecting the data, any mutation to the data
silently rewrites the form's shape.

Disclosing structure is explicitly fine. Per the root `CLAUDE.md`, field-level
`visible:` redaction "hides property **values only** — it makes no claim to
conceal *which* properties exist, since the metamodel (declared property names
per type) is served over the API. A 'field-existence oracle' is not a threat
this guards against." (Row-level ACL is a genuine secret and is untouched here.)

> **What actually shipped** differs from the plan below in two ways, both
> decided after review. The plan is kept for the reasoning trail; this note is
> the authority on the final state.
>
> 1. **No `visible: false` tombstone.** Step 1 proposed one. While this ticket
>    was open, #1277 (BUG-MLT9DE, decision DEC-T0XIWQ) landed on develop with
>    the same diagnosis and a better mechanism: `_redacted: []string` naming the
>    withheld properties. It keeps `_fields` about affordances, mirrors the
>    existing `Inaccessible` field, and unifies the flat and wizard gates into
>    one tested predicate. The tombstone PR was closed in its favour, and steps
>    1-2 below are satisfied by that work, not by this one.
> 2. **`clear_when_hidden` is `no | yes`, not `no | yes | confirm`.** The
>    interactive `confirm` policy in steps 5-7 was built and then removed: it
>    needs the form to separate "proposed" from "committed", which the current
>    architecture cannot express. Three fixes each passed their tests and each
>    then failed in manual use. The value is now rejected at config-validation
>    time rather than shipped half-working. See RR-PZHJNN, and TKT-7S5735 for
>    the refactor that unblocks it.
>
> What shipped is steps 3, 4, 8 and 9: retention in a separate ref, no eager
> `scheduleUnset`, create path untouched, and the mechanism in its own
> composable.

> **Original plan, revised after design review.** Twelve findings (RR-2IJL4Y,
> RR-RK7ATG, RR-805VYU, RR-F3Y9QA, RR-VFQKCY, RR-2S0333, RR-DSAN9B, RR-SH85S4,
> RR-O0KRI2, RR-IOZI72, RR-WRUTYN, RR-9BET7H).

1. **Add a `visible: false` tombstone to `_fields`.** `Visible *bool` on
`FieldAffordance` (`internal/apiwire/v1/responses.go:73`), same sparse
convention as `Writable *bool`; `affordances.go:740` emits it instead of
`continue`-ing. This is what makes the rest simple: "absent" stops meaning
"hidden", so nothing downstream has to infer intent from a missing key. Retires
the ambiguous sentinel why3/why5 name as the root cause.
2. **Render structure from the metamodel**, with a flat two-line rule:
`_fields[p].visible === false` → render read-only *redacted* affordance
(precedent: `InaccessibleField.vue`); else declared in the metamodel → render
normally. No inference from absence.
3. **Retain hidden values in a SEPARATE `retainedHidden` ref**, never in
`formData`. Keeps `formData` meaning exactly what it means today, so
`mergeServerResponse`'s disappeared-key sweep (`useAutoSave.ts:521-526`),
`pruneWizardHidden`, `checkDirty` and the create path are all untouched.
4. **No eager `scheduleUnset`** in the visibility watcher. A UI visibility
change must not mutate server state as a side effect. The watcher keeps its
RR-U9ERK error-clearing effect; delete the "edit analogue of create's prune"
comment (`:200-204`) — that comment *is* the unsound symmetry argument.
5. **New PER-FIELD config key `clear_when_hidden: no | yes | confirm`**,
default `no`. No step-level key (`FormStep` has no per-field behavior keys today
and this ticket does not invent config surface) — a step hiding is simply "all
its fields hid", each honoring its own setting. Not on `FormRelation` either.
Allowlist-validated in `validateFormField` (`validate.go:276`); frontend type is
a union, not `string`.
   - `no` — hide, retain client- and server-side, never prompt.
   - `yes` — hide and clear (today's behavior, now opt-in).
   - `confirm` — prompt; **yes** → clear; **no** → revert the trigger field.
6. **`confirm` gates before write emission.** Nothing is staged into `pending`
and no debounce timer is armed until the dialog resolves, so declining is a true
no-op against the server — no in-flight write to cancel, no race.
   - Restore from an **explicit pre-change snapshot** captured at gate time,
NOT `revertField`, whose `lastSeenServer` baseline can rewind an unrelated
already-accepted edit (RR-VFQKCY).
   - The re-prompt guard is a **generation counter or `nextTick()`-cleared
flag**, never a synchronously-cleared boolean — the watcher is post-flush and
would clear it before observing the revert (RR-2S0333).
   - Reuse `useConfirm`/`ConfirmModal` as-is; its `.modal-overlay` already
blocks the form. Add one `pendingGate` re-entry guard for keyboard/ same-field
re-toggle (RR-RK7ATG).
7. **Prompt only when something is at stake** — a hidden field holding a
non-empty stored value, where emptiness is `isClearedForType` (so boolean
`false` is a real value) and "stored" includes `pending`. One batched dialog
naming each field and value, never one per field. **Direct hides only** — no
transitive closure, no hypothetical-evaluation API.
8. **Create path unchanged.** RR-O4SRG's drop-on-commit stands.
9. **Land the mechanism as a composable** (`useHiddenFieldPolicy` or similar),
not inline — `DynamicForm.vue` is already 1886 lines against a 500-line
threshold. This also gives the currently-absent unit tests a seam.

Note `confirm` is not merely "`yes` with a prompt" — it can also undo the
triggering change, which `yes` never does. Document that explicitly.

### Accepted limitation

With deeply-chained conditions (`c` conditioned on `b` conditioned on `a`), a
dependent field may stay visible holding a stale value after its parent branch
hides. Strictly better than today's silent permanent loss, rare in practice, and
`clear_when_hidden: yes` is the escape hatch. Document; do not engineer around
it.

### Out of scope

Recovery of already-corrupted data. Prior values survive in postgres
`entity_versions` / git history, but no tooling is added here; the release note
should point at those paths. Existing deployments relying on today's auto-clear
will change behavior on upgrade — intended, and called out in the release note.
No migration: `internal/migration/` holds schema-shape migrations only, and
auto-adding `clear_when_hidden: yes` would be a migration that preserves the
bug.
