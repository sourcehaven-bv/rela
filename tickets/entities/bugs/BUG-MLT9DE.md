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
status: done
---

## Description

When a property is added to an entity type in `metamodel.yaml` (and to the
relevant data-entry form config), the edit form did not render it for entities
created before that change. Because the field never rendered it couldn't be
filled in; because it couldn't be filled in the entity never got the property;
so the field never rendered. Existing entities were permanently stuck without
the new field, and the only way out was the CLI or editing the markdown by hand.

Found on 8cce2733 (`frontend/src/components/forms/DynamicForm.vue`).

## Reproduce

1. Have an entity type with existing entities.
2. Add a new optional property to that type in `metamodel.yaml` and add it to
the form's `fields:` in the data-entry config.
3. Open the edit form for an entity created before step 2 — the new field is
absent.
4. Creating a *new* entity of that type shows the field correctly.
5. `rela update <ID> -P newprop=value` → the field now appears for that entity.

Step 5 is decisive: it changes *only* whether the key exists in the entity's
stored `properties`.

## Cause

The affordance filter added by TKT-G7N5 / TKT-3I5U gated rendering in two places
— `fields` and `affordanceVisible()`:

```js
if (f.property in fieldAffordances.value) return true
if (isEdit.value && f.property in formData.value) return true   // ← gate
return false
```

In edit mode a configured field rendered only if the key was already in
`formData` (the entity's stored properties) or listed in `_fields`. `_fields` is
sparse — a plainly writable field is absent from it by design — so the presence
check became the sole gate. Reproduces with the nop resolver (`_fields: {}`), so
it was **not** ACL-dependent; it affected every deployment.

The gate treated absence from `properties` as proof of redaction. But absence
means *either* "redacted" *or* "never set", and the wire could not distinguish
them, by design:

- `stripHiddenProperties` deletes the key outright — no null, no tombstone.
- Hidden fields are *also* omitted from `_fields` — the documented
"doubly-invisible" contract.
- `Entity.Inaccessible`, the one wire field meaning "exists but unreadable", is
populated only for git-crypt–locked content, never for ACL redaction.

The "doubly-invisible" contract is right for read-out surfaces, where hidden and
unset *should* look alike. A write form has the opposite need — it must know
which fields it may offer — and the contract was being reused across that
boundary.

## Why create mode was unaffected

Create mode uses `stagedVisibleProps`, built from the dry-run candidate, and the
server applies metamodel defaults when staging it. Every declared property is
therefore a key in the candidate — a genuine visible-set rather than an
inference from absence. Edit mode had no equivalent source.

## Impact

Any additive schema change — the most routine kind of evolution — was invisible
in the UI for pre-existing data, requiring a scripted backfill or hand-editing
every file. It also failed silently: no warning, no empty field, the input
simply wasn't there, which read as a config error rather than a product bug.

## Resolution

Fixed at the source rather than in the inference (DEC-T0XIWQ): the server now
states which properties it withheld, in a new `_redacted` field on per-entity
responses, and the SPA stops guessing.

- `internal/apiwire/v1/responses.go` — `Entity.Redacted`, same closed-world
pointer semantics as `_fields` (present-possibly-empty per-entity, absent on
list rows).
- `internal/dataentry/affordances.go` — `redactedPropertyNames`, attached in
`attachEntityAffordances` beside the strip, so naming and stripping are
structurally inseparable.
- `frontend/.../DynamicForm.vue` — the two hand-synced gate copies collapsed
into one predicate; edit mode renders a configured field unless `_redacted`
names it. (The wizard path carried the identical defect precisely because the
rule was duplicated.)
- `frontend/src/utils/affordances.ts` — `isPropertyRedacted`, exported and
directly tested.
- `docs/data-entry/api-reference.md` — the hidden-fields section documented the
unsound inference *as the contract*, so it would have kept teaching the bug;
rewritten.

Values are still withheld; only names became explicit, which discloses nothing
new — the metamodel endpoint already serves declared property names, and
`visible:` redaction is defined as hiding values only. Row-level ACL is
untouched.

**Note on the originally suggested fix.** The report proposed keying on
"declared in the metamodel". That premise doesn't hold: `allFields` is built
from the data-entry form config and never consults the metamodel property map,
so such a fix would have been both wider than needed and wrong for forms that
deliberately omit a property. See BUGA-YBCFE1.

**Note on the anticipated data-loss risk.** Planning flagged that rendering
redacted fields might let an untouched one submit as empty and clobber the
stored value. Implementation found this cannot happen: edit mode has no bulk
submit, and all writes go through per-property autosave, which fires only for
fields the user typed into. A guard written against that path was removed rather
than left as dead code implying protection it didn't provide; two tests pin the
reasoning, and an unreachable branch that contradicted it was deleted
(RR-GNXKTW).
