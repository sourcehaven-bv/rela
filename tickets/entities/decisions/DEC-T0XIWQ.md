---
id: DEC-T0XIWQ
type: decision
title: Redacted properties carry an explicit wire signal; the SPA never infers hiding from absence
context: 'The v1 entity wire made an ACL-redacted property byte-identical to a never-set one: stripHiddenProperties deletes the key (affordances.go:913) and hidden fields are also absent from the sparse _fields map (affordances.go:741,752) — the documented ''doubly-invisible'' contract. The SPA''s edit-mode form filter therefore had to INFER hiding from absence (''render only if the field appears in _fields OR properties'', api-reference.md:439-443). That inference is unsound because absence has two causes, and it resolved them toward hiding — making any unset property permanently unreachable in the edit form (BUG-MLT9DE). The doubly-invisible contract is right for read-out surfaces, where hidden and unset SHOULD look alike; a write form has the opposite need — it must know which fields it may offer.'
consequences: 'Adds `_redacted: []string` to the per-entity wire shape (present alongside _fields, same closed-world pointer semantics), naming properties withheld by field-level ACL. The SPA''s gate sites — unified into ONE predicate, isPropertyRedacted, instead of two hand-synced copies — stop inferring and render a configured field unless it is positively named redacted. It narrows the read-side contract: field NAMES (already public via the metamodel endpoint) become explicit, while VALUES stay withheld; per the project''s field-level ACL rule this is not a new disclosure, since visible: redaction never claimed to conceal which properties exist. Row-level hiding is untouched — a hidden ENTITY remains a genuine secret. api-reference.md''s hidden-fields section was rewritten: it previously documented the unsound inference as the contract.'
date: "2026-08-06"
status: accepted
---

## Context

See the `context` property. The short version: the wire deliberately erased a
distinction that a write surface needs, so the client reconstructed it by
guessing, and guessed wrong in the common case.

## Decision

Make the server say what it means.

1. **Add `_redacted: []string`** to `v1.Entity` — the property names withheld
by field-level ACL on this response. Same closed-world pointer semantics as
`_fields` / `_relations`: present (possibly empty) on per-entity responses,
absent on list rows. It is the field-level sibling of the existing
`Inaccessible` field, which already expresses exactly this idea ("exists but
unreadable") for git-crypt-locked content.

2. **The SPA stops inferring.** The render gate renders a configured property
field unless it is positively named in `_redacted`. Absence from `properties`
stops meaning anything.

3. **Keep `stripHiddenProperties` exactly as is.** Values are still withheld.
Only the *names* become explicit.

4. **One predicate, not two.** The flat (`fields`) and wizard
(`visibleStepFields`) paths previously held hand-synced copies of the gate —
which is why the wizard silently carried the same defect. Both now delegate to
`affordanceVisible`, which delegates to the exported, directly-tested
`isPropertyRedacted`.

## Why not the client-only fix

Rendering every configured field and relying on the PATCH gate would fix the
symptom with less code. Rejected because it leaves the ambiguity in place: the
next consumer of this wire shape faces the same unsound inference, and the
failure mode is silent both times.

## Security analysis

The disclosure delta is **property names, not values**, and it is not new:

- The metamodel endpoint already serves the declared property names per type.
`visible:` redaction is explicitly documented as hiding property *values* only,
making "no claim to conceal which properties exist" — a field-existence oracle
is not in this threat model (root CLAUDE.md, field-level ACL rule).
- Row-level ACL is untouched and remains fail-closed: whether an *entity*
exists is a genuine secret, and `_redacted` only ever appears on a response the
caller was already authorized to read.
- `_title` fallback stays — the display-title channel must not leak a hidden
value.

Net: no value ever becomes readable that was not readable before. Pinned by
`TestV1Affordance_PerEntityGet_RedactedNamesHiddenFields` and the
strip/`_redacted` agreement test.

## Write-path note (corrected during implementation)

Planning flagged a data-loss risk: if redacted fields render, an untouched one
might submit as empty and clobber the hidden stored value. **Implementation
found this cannot occur on the edit path.** Edit mode has no bulk submit —
`handleSubmit` returns early when `isEdit`, and every write goes through
per-property autosave, which fires only for a property the user actually typed
into. An untouched field is never in a payload at all, and a redacted field
additionally never renders to be typed into.

A guard written against the bulk path was therefore removed rather than left in
as dead code implying protection it did not provide. Two tests pin the reasoning
so a future bulk-submit path cannot silently reintroduce the risk: one asserts a
form submit in edit mode sends nothing, one asserts a redacted field does not
render.

## Docs

`docs/data-entry/api-reference.md`'s hidden-fields section documented the
unsound inference as the contract ("rendered only if it appears in `_fields` OR
`properties`"). Rewritten to describe `_redacted` and to state plainly: never
infer redaction from absence.
