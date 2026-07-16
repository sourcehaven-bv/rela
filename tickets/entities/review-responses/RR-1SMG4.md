---
id: RR-1SMG4
type: review-response
title: 'CREATE has no old value: initial-state entry into the machine is unspecified'
finding: 'CreateEntity runs ACL then ValidateEntity with no GetEntity pre-read (manager.go:334-411; comment at 350 notes ''No GetEntity pre-check''). The design''s `initial:` field and ''only legal entry state'' AC have nothing to diff against on create. Undefined: is setting status to a non-initial value on create rejected? Is the initial-state entry itself guardable (who may create an entity already in `approved`)? The [*]->initial edge from the mermaid analogy is unspecified for the create path.'
severity: significant
resolution: 'Create-path entry semantics implemented: value defaults to Initial-else-Default; a non-initial explicit value on create is rejected 422 (ErrIllegalEntry) via EnforceCreate, now run pre-persist in createCore. Tests TestTransition_IllegalEntryOnCreateIs422, TestTransition_IllegalEntry_DoesNotPersist.'
status: addressed
---

## Finding

`CreateEntity` (`manager.go:334-411`) has no old-state read (comment line 350:
"No GetEntity pre-check"). The design mentions `initial:` and "only legal entry
state" but the create path gives nothing to diff against.

## Undefined behavior to specify

1. On create, may `status` be set to any value, or only `initial`? (The mermaid
`[*] --> initial` edge implies only `initial`.)
2. Is entry into the machine itself guardable — e.g. may only certain principals
create an entity that starts in a non-default state?
3. If a create sets a non-initial state, is that a 422 (illegal entry) or
silently allowed?

## Resolution

Add explicit create-path semantics: default is `initial` (or the enum default);
setting any non-`initial` value on create is either rejected (recommended,
matches "only legal entry state") or requires the guard of the `[*]->value` edge
if one is modeled. Make this an acceptance criterion — it's currently a gap, not
a decision.
