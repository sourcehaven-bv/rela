---
id: TKT-OMUD56
type: ticket
title: Inline entity creation from relation fields in the data-entry edit form
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

When creating or editing an entity, a relation field should let the user create
a brand-new target entity **inline** (in a modal) and link it, without
navigating away and losing the in-progress draft.

## The rule

A user gets inline create for a target type when **both** hold:

- **A — permission**: the principal may `create` that entity type (`acl.OpCreate`
against `EntitySubject{Type: X, ID: ""}`).
- **B — a form resolves**: `createFormForType` resolves a form for that type.
Per its doc comment it prefers a non-edit form and **falls back to an edit-mode
form**, which works for creation when no entity id is supplied — so condition B
is "a form exists", not "a non-edit form exists" (RR-KGCF61).

No config gate. `FormRelation.AllowCreate` and `FormRelation.CreateForm` are
**deleted**, not wired up — both conditions are computed, not declared.

## What this replaced

- `frontend/src/components/forms/InlineCreateModal.vue` (417 lines) was reachable
only from `RelationPicker`'s `+ Add new <Type>` button and **bypassed the
data-entry form config entirely**: it iterated `entityTypeDef.properties` off
the metamodel with an ad-hoc widget dispatch — no form definition, templates,
validation beyond HTML `required`, wizard steps, relation fields, or dry-run
affordance check. It stripped falsy values on submit (`if (value !== '' &&
value !== false)`), so an intentional `false` boolean was never sent, and it
had no unit test file. **Deleted.**
- `RelationCards.vue` (the edge-meta widget) had **no inline-create affordance at
all**. It has one now, routed through `selectTarget` so required edge
properties can still be filled before linking.
- `FormRelation.AllowCreate` / `CreateForm` were declared and validated but
**read by nothing**. **Deleted** — no migration needed, since unknown nested
YAML keys are silently ignored and no config in the repo set them.
- The old `+ Add new` button was gated on `verdict.creatable`, the
**relation-edge** permission — unrelated to entity-create permission for the
target type. That gate remains (you must be able to add the edge) and is now
AND-ed with the entity-create gate; they are orthogonal.
- The e2e placeholder `Inline Entity Creation → …` was vacuous: its assertions
sat inside an `if` whose selectors never matched the real markup, so it passed
while testing nothing. Replaced by six real tests.

## Where each signal comes from

Settled during design review (RR-R1I6AD), then refined in implementation: the
server answers **both** conditions at once by sending, per eligible entity type,
the *resolved form id* — so presence in the map IS the affordance and the client
does no permission arithmetic.

| Signal | Source |
| --- | --- |
| A: may-create type X | `computeCollectionActions` (`affordances.go:135`) — the same call the list handler uses, so verdicts cannot diverge |
| B: a form for type X | `createFormForType` (`views_handler.go`) reused unchanged |

Both are combined in `viewsHandler.inlineCreateForms` and served as
`SidebarResponse.inline_create`. The sidebar carries it because it is the one
boot-time payload that is already principal-scoped: `_config` is pinned
principal-INDEPENDENT (`nav_permission_test.go`) and `_schema` is a pure
metamodel projection with no access to the config-layer resolver.

Sending the resolved id (rather than letting the SPA find its own form) keeps
`createFormForType`'s natsort-and-prefer-non-edit ordering authoritative in one
place — the client would otherwise reimplement it and could silently diverge.

**Note**: `create` implies no `read` (`internal/acl/policy.go:239-243`) — a role
may create a type it cannot read. The permission map must come from
`computeCollectionActions` and must never be inferred from sidebar visibility or
from which lists returned rows. POST returns the created entity via `forWire`,
so both widgets seed their caches from it; only a later re-fetch 404s, which
must degrade to showing the id (AC11).

## Derivation precedent (settled)

Lists and kanbans do **not** derive a create form — `List.CreateForm` /
`Kanban.CreateForm` are operator-set, and the "+ New" button simply doesn't
render when unset. Only side-panel sections derive, via `createFormForType`.
Since the rule above makes inline create *derive*, `createFormForType` is the
shared resolver, keeping the two deriving surfaces consistent.

## Key constraints discovered

- **A nested create must never navigate.** Create-mode draft state lives entirely in
`DynamicForm.vue` component refs with no persistence; any route push unmounts
the form and destroys the draft. Modal only.
- `useConfirm` is a singleton with a single in-flight promise — a nested form must
not register `onBeforeRouteLeave`, or one dialog silently decides for both
forms.
- `DynamicForm` registers a **document-global** `keydown` (Cmd/Ctrl+Enter submit);
two instances both fire, submitting the parent form. It does not consult
`isAnyModalOpen()`.
- `useFormWizard` writes `?step=N` to the URL (`useFormWizard.ts:211`); a nested
wizard and a parent wizard fight over the same query key.
- `RelationCards` requires `entityId`, so a nested create form renders only
`RelationPicker` for its own relations. Nesting is capped **structurally** by an
injected `inlineCreateDepth`, not left to that accident (RR-UTURB1).
- `.dynamic-form` has `min-width: 500px` vs `.modal-content` `max-width: 480px`.
- No modal implements a focus trap (`TKT-X4P99`, backlog).
