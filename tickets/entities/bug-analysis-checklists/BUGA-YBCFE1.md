---
id: BUGA-YBCFE1
type: bug-analysis-checklist
title: 'Analysis: Edit form hides properties the entity doesn''t have yet — newly added metamodel properties are unreachable on existing entities'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally — by code inspection + the reporter's confirmed repro (step 5, `rela update -P newprop=value` makes the field appear, is decisive: it changes *only* whether the key exists in the entity's stored `properties`)
- [x] Minimal reproduction steps documented — see the bug body. Minimal form: an entity whose stored `properties` omit key `K`, where `K` is listed in the form config's `fields:`, served with `_fields: {}`.
- [x] Environment/conditions noted — 8cce2733; reproduces with the nop ACL resolver (`_fields: {}`), so it is **not** ACL-dependent. Affects every deployment.

### Scope correction found during analysis

The bug report's suggested fix ("the schema store already has the type's
property list") rests on a premise that does not hold. `allFields`
(DynamicForm.vue:151-167) is built from the **data-entry.yaml form config**
(`formConfig.fields` / `steps[].fields`), never from the metamodel property map;
`dataentryconfig` has no derive-fields-from-metamodel path and no lint requiring
a form to cover every declared property.

Consequence: adding a property to `metamodel.yaml` alone renders it nowhere —
not create, not edit. The reporter saw it in create mode, so the property was
already added to the form config too. The metamodel is therefore **not** the
missing signal; the affordance filter subtracting a configured field is the
whole bug. A fix keyed on "declared in the metamodel" would be both wider than
needed and, for a form that deliberately omits a property, wrong.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

### Evidence

The edit-mode gate treats absence of a key in `properties` as proof of
ACL-hiding. That inference is invalid because the wire genuinely cannot
distinguish the two cases:

- `stripHiddenProperties` (affordances.go:913-916) removes a hidden property
with a plain `delete` — no null placeholder, no tombstone.
- Hidden fields are also **absent** from `_fields` by construction: both loops
in `computeFieldAffordancesFrom` skip them (affordances.go:741-743, 752-754),
locked by `TestComputeFieldAffordances_HiddenFieldsOmittedFromMap`
(affordances_test.go:497). Documented as "doubly-invisible"
(affordances.go:714-722).
- The one wire field that *does* mean "exists but unreadable",
`Entity.Inaccessible` (apiwire/v1/responses.go:23,123-129), is populated only
for git-crypt–locked content (entityserializer.go:61-69), never for ACL
redaction.

So "redacted" and "never set" are byte-identical on the wire, and the client
resolves the ambiguity in the direction that silently destroys reachability.

Create mode escapes only by accident of a different signal: `stagedVisibleProps`
(DynamicForm.vue:547) is seeded from the dry-run candidate, and the server
applies metamodel defaults when staging — so every declared property is a key.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

### Approach

Invert the edit-mode default from deny to allow, and hide only on positive
evidence of hiding. A configured form field is an explicit authoring decision
that the field belongs on the form; the affordance filter should subtract from
it only when the server actually says "hidden", never on inference from absence.

Two candidate levers:

1. **Client-only (preferred for the fix, smaller blast radius).** In edit mode
render a configured property field unconditionally, and rely on the existing
`isFieldReadonly` / `_fields.writable` path plus the server's PATCH gate for
enforcement. Correct per the project's field-level ACL rule: `visible:`
redaction hides property *values only* and makes no claim to conceal which
properties exist, since the metamodel is served over the API — so rendering an
empty input for a redacted field is not a leak. It must not *submit* an
unmodified empty value, though (see risk below).

2. **Add an explicit wire signal** — have the server name redacted properties
(mirroring `Inaccessible`) so the client stops inferring. Strictly better
long-term and removes the ambiguity at its source, but it is a wire-contract
change and larger than this bug needs. Recommend recording it as a follow-up
ticket rather than folding it in.

Both gate sites must change together: `fields` (DynamicForm.vue:249-262) and
`affordanceVisible()` (DynamicForm.vue:692-701, the wizard path). Fixing one
leaves the other broken.

### Load-bearing risk (must be handled in implementation)

A redacted field would now render as an empty input. If the form submits that
empty value on save, it **overwrites the hidden stored value with empty** —
silent data destruction, the exact failure the project's "never redact a read
that feeds a write" rule exists to prevent. The fix must ensure an untouched,
never-populated field is omitted from the PATCH payload rather than sent as
empty. `userTouched` (DynamicForm.vue:312) already tracks this distinction and
is the natural hook.

### Regression tests planned

Pinned as `edit-form-renders-unset-declared-property-test`. There is currently
**no `DynamicForm.test.ts` at all** — the only DynamicForm test
(`DynamicForm.guard.test.ts`) deliberately avoids mounting the component and
replicates the guard in a stub, so neither gate site has ever been covered.
Three cases minimum:

1. Declared-and-configured but unset property, `_fields: {}` → renders, writable,
and a typed value persists. (Fails today.)
2. Genuinely redacted property → still not writable, and critically **not sent
as empty on save** when untouched. Guards against trading a silent-hiding bug
for a silent-data-loss one.
3. The same pair through the wizard path (`visibleStepFields`), since that is a
second independent copy of the gate.

### Related areas checked

- Wizard gate `affordanceVisible()` — same defect, same file. In scope.
- `visibleWritablePropertiesForCommit()` (DynamicForm.vue:300-320) — create-path
only (early-returns in edit mode); not the bug, but it is where the
`userTouched` precedent for the empty-overwrite risk lives.
- List rows / `_props` (`copyVisibleProperties`, affordances.go:891-902) — strips
the same way, but read-only display, so absence is harmless there.
- Backend `_fields` contract — correct as designed and well tested; no change
needed for approach 1.
