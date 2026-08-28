---
id: edit-form-renders-unset-declared-property-test
type: automated-measure
title: Edit form renders a configured-but-unset property field
description: 'Test suite pinning the redacted-vs-unset distinction end to end. Frontend: DynamicForm.test.ts mounts the form in edit mode with an entity whose stored properties OMIT a configured property and _fields: {} (the sparse permissive default), asserting it renders — plus the inverse (a _redacted property does not render) and both cases on the wizard path. Backend: api_v1_test.go asserts a GET states hidden NAMES in _redacted while withholding VALUES, that an unset property is named nowhere, and that _redacted is empty-not-nil under the permissive default. Verified to fail on pre-fix code.'
kind: test
location: frontend/src/components/forms/DynamicForm.test.ts, frontend/src/utils/affordances.test.ts, internal/dataentry/api_v1_test.go, internal/dataentry/affordances_test.go
status: active
---

## Purpose

Close the test gap that let BUG-MLT9DE ship. Before this there was **no
`DynamicForm.test.ts` at all** — the only DynamicForm test
(`DynamicForm.guard.test.ts`) deliberately avoids mounting the component and
replicates the unsaved-changes guard in a stub. Neither affordance gate site had
ever been exercised. Fixtures elsewhere uniformly seeded entities whose
`properties` already contained a key for each field under test, so the edit-mode
gate always matched and its failure mode was invisible to CI.

## What it covers

**Frontend — `DynamicForm.test.ts` (7 cases)**

1. *The bug.* Edit mode, entity whose `properties` omit a property that IS in
the form config's `fields:`, served with `_fields: {}` — the sparse permissive
default the old gate misread as "hidden". Asserts it renders.
2. *The inverse.* A property named in `_redacted` still does not render, so the
fix does not trade silent hiding for silent exposure.
3. *Unset ≠ redacted.* With one property redacted, the merely-unset one still
renders — the distinction the whole change exists to make.
4. *Wizard path.* Both cases again through `visibleStepFields`, which carried a
hand-synced copy of the gate.
5. *No bulk overwrite.* A form submit in edit mode sends nothing (writes are
per-property autosave), so no untouched field can post as empty.

**Frontend — `affordances.test.ts`**: `isPropertyRedacted` unit cases, including
the two that encode the contract — `[]` hides nothing, `undefined` hides
nothing.

**Backend — `api_v1_test.go` / `affordances_test.go`**: a GET states hidden
names in `_redacted` while withholding values; an unset property is named
nowhere; `_redacted` is empty-not-nil under the permissive default; list rows
omit it; and `_redacted` agrees exactly with what `stripHiddenProperties`
removed (the invariant tying the two halves of the wire together).

## Verified against the bug

Reverting the edit-mode gate to its pre-fix form fails 4 of the 5 render cases,
including the wizard one. The 5th passed only because the old code hid
*everything* unset — which is why the paired render/hide assertions matter:
either alone can pass for the wrong reason.

## Note on scope

Keyed on the **form config** field list, not the metamodel: `allFields` is built
from `formConfig.fields` / `steps[].fields` and never consults the metamodel
property map. See BUGA-YBCFE1.

## Structural follow-through

The duplicated gate sites are now one exported predicate (`isPropertyRedacted`),
so this class of drift is prevented structurally rather than only test-caught.
