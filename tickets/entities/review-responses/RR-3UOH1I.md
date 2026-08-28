---
id: RR-3UOH1I
type: review-response
title: 'Minor design gaps: _actions naming on a type, visible_when parent context, cards Link-cancel orphan, modal layout inheritance'
finding: 'Four smaller issues. (1) Putting _actions on EntityType would be a third scope for a name that currently means per-item verbs or per-collection create — risks colliding with a future per-type meaning. (2) AC4 promises visible_when works in the nested form, but conditions evaluate against the nested form''s own formData, so the natural operator expectation (hide when the PARENT''s status is X) is unsupported and undocumented. (3) The plan covers orphan-on-save-failure but not the likelier RelationCards path: create → selectTarget → cancelAdd() (RelationCards.vue:373-379) leaves the entity persisted and unlinked, and the plan suppresses the success toast so nothing tells the user it exists. (4) .dynamic-form min-width 500px, the .form-layout.with-sidepanel selector, and the suppressed header must not leak into the modal.'
severity: minor
resolution: '(1) Naming moot after RR-R1I6AD — the map is no longer on EntityType. (2) AC4 reworded: visible_when sees only nested data; parent-context conditions explicitly unsupported and documented. (3) Modal reports "Created <title> — link it below" so a later cancelAdd() doesn''t hide that the entity exists; added to edge cases. (4) Modal scopes its own width and must not inherit .with-sidepanel; mobile-viewport check added to the e2e plan.'
status: addressed
---

## Resolution

1. **Naming** — moot after RR-R1I6AD: the per-type create map no longer goes on
`EntityType`. It ships as an explicitly-named `create` permission map on the
principal-scoped payload, avoiding a third meaning for `_actions`.
2. **`visible_when`** — AC4 reworded: conditions evaluate against the *nested*
form's own data. Parent-context conditions are explicitly unsupported and called
out in `docs/data-entry.md`, since the operator expectation runs the other way.
3. **Link-cancel orphan** — the modal's close signal states the entity was
created ("Created <title> — link it below") instead of closing silently, so a
subsequent `cancelAdd()` does not hide the fact that the entity exists. Added to
the edge-case list alongside the save-failure orphan.
4. **Layout** — the modal scopes its own width and must not inherit
`.form-layout.with-sidepanel`; verified `SidePanel` is `isEdit`-gated so it
never renders nested. Added a mobile-viewport check to the e2e plan for the
`.mobile-actionbar` fixed positioning.
