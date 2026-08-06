---
id: BUG-MLT9DE
type: bug
title: Edit form hides properties the entity doesn't have yet — newly added metamodel properties are unreachable on existing entities
description: In edit mode DynamicForm renders a property field only if the key is already present in formData or in the sparse _fields map, so a property added to metamodel.yaml after an entity was created never renders and can never be set from the UI.
priority: high
effort: m
why1: In edit mode DynamicForm renders a configured property field only if its key is already present in the entity's stored properties (or in the sparse _fields map), so a property that is simply unset never renders and can never be filled in.
why2: The gate treats absence of a key in the GET response's `properties` as proof the server redacted it, but absence actually means either 'ACL-redacted' or 'never set' — the filter resolves that ambiguity toward hiding.
why3: 'The wire genuinely cannot distinguish the two: stripHiddenProperties deletes the key outright (affordances.go:913), and hidden fields are also omitted from `_fields` by construction (affordances.go:741,752) — the deliberate ''doubly-invisible'' contract. Entity.Inaccessible, the one field that means ''exists but unreadable'', is used only for git-crypt locks, never ACL.'
why4: The client was made responsible for reconstructing a distinction the server had deliberately erased. The 'doubly-invisible' contract was designed for read-out surfaces, where a hidden and an unset field should look identical; a write form has the opposite requirement — it must know which fields it may offer — and no one noticed the contract was being reused across that boundary.
why5: 'Systemic: an affordance filter added for ACL (TKT-G7N5/TKT-3I5U) defaulted to DENY on a signal that is absent in the overwhelmingly common no-ACL case, and nothing tested it there. There is no DynamicForm.test.ts at all, and every fixture elsewhere seeds entities whose properties already contain each field under test — so the filter''s failure mode was invisible to CI, and a security-motivated default silently became a functional regression for all users.'
prevention: 1) When a read contract deliberately makes two states indistinguishable, do not consume it from a write surface that needs to tell them apart — carry an explicit signal (mirroring Entity.Inaccessible) instead of inferring. 2) An ACL filter that defaults to deny must be tested in the NO-ACL configuration, where the permissive default is what ships to most users. 3) Test fixtures must include an entity with unset properties; uniformly fully-populated fixtures hide every absence-related defect. 4) Duplicated gate logic (fields + affordanceVisible) should be one exported function so it is testable and cannot drift.
status: in-progress
---

## Description

When a property is added to an entity type in `metamodel.yaml` (and to the
relevant data-entry form config), the edit form does not render it for entities
created before that change. Because the field never renders it can't be filled
in; because it can't be filled in the entity never gets the property; so the
field never renders. Existing entities are permanently stuck without the new
field, and the only way out is the CLI or editing the markdown by hand.

Found on 8cce2733 (`frontend/src/components/forms/DynamicForm.vue`).

## Reproduce

1. Have an entity type with existing entities.
2. Add a new optional property to that type in `metamodel.yaml` (e.g. an enum
with a `default:`) and add it to the form's `fields:` in the data-entry config.
3. Open the edit form for an entity created before step 2 — the new field is
absent.
4. Creating a *new* entity of that type shows the field correctly.
5. `rela update <ID> -P newprop=value` → the field now appears in the edit form
for that entity.

Step 5 is the decisive one: it changes *only* whether the key exists in the
entity's stored `properties`.

## Cause

The affordance filter added by TKT-G7N5 / TKT-3I5U gates rendering in two places
— `fields` (DynamicForm.vue:249) and `affordanceVisible()`
(DynamicForm.vue:693):

```js
if (f.property in fieldAffordances.value) return true
if (isEdit.value && f.property in formData.value) return true   // ← gate
return false
```

In edit mode a configured property field renders only if the key is already
present in `formData` (populated from the entity's stored properties) or listed
in `_fields`.

`_fields` cannot cover it: `computeFieldAffordances`
(`internal/dataentry/affordances.go:714`) is sparse — *"only fields whose
verdict deviates from the permissive default appear"*. A plainly writable field
is therefore absent from `_fields` by design, so the first check never matches
and the presence-in-`formData` check becomes the sole gate. Reproduces with the
nop resolver (`_fields: {}`), so this is **not** ACL-dependent — it affects
every deployment.

The gate's intent was to mirror the server's redaction ("hidden fields are
stripped server-side, so absence means hidden"). But absence from `properties`
is ambiguous — it means *either* "redacted" *or* "never set" — and the gate
reads every never-set property as redacted.

The wire cannot distinguish them, by design:

- `stripHiddenProperties` (affordances.go:913-916) deletes the key outright —
no null, no tombstone.
- Hidden fields are *also* omitted from `_fields` (affordances.go:741-743,
752-754), locked by `TestComputeFieldAffordances_HiddenFieldsOmittedFromMap`.
Documented as "doubly-invisible" (affordances.go:714-722).
- `Entity.Inaccessible` (apiwire/v1/responses.go:23), the one wire field meaning
"exists but unreadable", is populated only for git-crypt–locked content
(entityserializer.go:61-69), never for ACL redaction.

## Why create mode is unaffected

Create mode uses a different signal: `stagedVisibleProps` (DynamicForm.vue:547)
is built from the dry-run candidate's `properties`, and the server applies
metamodel defaults when staging that candidate. Every declared property is
therefore a key in the candidate. Edit mode has no equivalent source.

## Why the metamodel `default:` doesn't help

Defaults are applied at create time only. Stored entities simply lack the key,
and the API omits it from `properties` rather than emitting a null.

## Impact

Any additive schema change — the most routine kind of evolution — is invisible
in the UI for pre-existing data. With a large existing dataset this means either
a scripted backfill or hand-editing every file. It also fails silently: no
warning, no empty field, the input simply isn't there, which reads as a config
error rather than a product bug.

## Suggested fix

**Note:** the original report suggested keying the fix on "property is declared
on the entity type in the metamodel". Analysis (BUGA-YBCFE1) found that premise
does not hold — `allFields` (DynamicForm.vue:151-167) is built from the
**data-entry form config**, never from the metamodel property map, and there is
no derive-from-metamodel path in `dataentryconfig`. A metamodel-keyed fix would
be both wider than needed and wrong for a form that deliberately omits a
property.

The correct lever: in edit mode, render a *configured* property field
unconditionally, and hide only on positive evidence of hiding — never on
inference from absence. Enforcement stays with `isFieldReadonly` /
`_fields.writable` and the server's PATCH gate. This is consistent with the
project's field-level ACL rule: `visible:` redaction hides property *values*
only and makes no claim to conceal which properties exist, since the metamodel
is served over the API.

Both gate sites must change together — `fields` (DynamicForm.vue:249) and
`affordanceVisible()` (DynamicForm.vue:693, wizard path).

**Load-bearing risk:** a redacted field would then render as an empty input. If
the form submits that empty value on save it overwrites the hidden stored value
— silent data destruction, exactly what the "never redact a read that feeds a
write" rule guards against. An untouched field must be omitted from the PATCH
payload, not sent as empty; `userTouched` (DynamicForm.vue:312) is the natural
hook.

Follow-up worth considering separately: give the server an explicit
redacted-properties signal (mirroring `Inaccessible`) so the client stops
inferring at all. Better long-term, but a wire-contract change and larger than
this bug needs.
