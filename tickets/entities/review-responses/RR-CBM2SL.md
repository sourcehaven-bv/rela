---
id: RR-CBM2SL
type: review-response
title: 'Bare `input[type=checkbox]` selector encodes "I am the only checkbox in this SFC" as an unenforced invariant'
finding: 'The rules key on a bare element selector. Vue''s scoped `data-v-` attribute confines them to this SFC, which today renders exactly one input — so there is no live blast radius. But the day someone adds a second checkbox to the component (a tri-state control, an "apply to all" affordance) it silently inherits all the rules. RelationCards used a real class (`.inline-edit-checkbox`) precisely because it is not alone in its file.'
severity: minor
resolution: 'Left as-is, deliberately.'
reason: 'The bare-element form is the house style for the widget set — TextWidget uses `input {`, SelectWidget uses `select {` — and these SFCs are single-control by construction. Switching this one widget to a class would make it the odd one out without removing any live risk. Verified there is no current exposure: single-root template, no <slot>, no child components, and no `:deep()` anywhere in src/widgets/. If a second checkbox is ever added here, that edit is the moment to introduce the class.'
status: wont-fix
---

Confirmed live rather than reasoned about: on the demo detail page the four
markdown task-list checkboxes rendered in the same DOM kept `appearance: auto`
at 13px, so the scoped rule did not reach past the widget.
