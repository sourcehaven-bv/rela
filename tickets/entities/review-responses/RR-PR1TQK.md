---
id: RR-PR1TQK
type: review-response
title: 'CSS cascade audit: scoped-vs-global precedence verified empirically, not assumed'
finding: |-
    The shared styles/properties-list.css is loaded globally from main.ts, but the components that consume it (PropertyDisplay, SidePanel) still carry SCOPED style blocks touching the same classes. Vue's scoped compilation appends an attribute selector, which RAISES specificity -- so a leftover scoped rule silently outranks the shared stylesheet. Combined with the equal-specificity pair in DynamicButton (.form-fields > * vs .form-field, both originally setting grid-column), this is the kind of cascade interaction that looks fine in a diff and breaks in a browser.

    Raised by the code reviewer; the review agent stalled mid-run before reporting, so the audit was completed by hand against the running app rather than left open.
severity: minor
resolution: |-
    Audited in the browser by enumerating every CSSRule matching the live elements, rather than reasoning about the source.

    1. SCOPED OVERRIDES ARE DELIBERATE AND DETERMINISTIC. Exactly two scoped rules remain on these classes: SidePanel's `.property-item dt` (denser label) and PropertyDisplay's `.property-item.property-long dd` (pre-wrap). Both compile to attribute-qualified selectors (e.g. `.property-item dt[data-v-dda0a2f9]`), so they win on specificity, not source order -- which is the intended relationship: the shared sheet owns layout and base typography, a component may deliberately override one property. Verified the side panel's dt resolves to 11px (its own rule) while its dd resolves to 14px (the SHARED rule), confirming the abstraction is real rather than a fake wrapper.

    2. THE EQUAL-SPECIFICITY HAZARD IS GONE BY CONSTRUCTION. After the fix, only ONE rule in the entire cascade sets grid-column on a form field: `.form-fields[data-v-7f10a9d8] > *`. The `.form-field` rule no longer declares grid-column at all, so there is nothing left to tie-break. This was confirmed by listing every matching rule with a grid-column declaration and finding exactly one.

    3. NO ORPHANED REDEFINITIONS. Grepped for any remaining `.properties-list`/`.property-item` declarations outside the shared sheet; only the two intentional overrides above and explanatory comments remain.

    Computed-style spot checks: detail page renders 12 tracks with `span 6` applied; side panel renders 1 track with `span 1`; text-transform resolves to `none` on both surfaces (the two uppercase rules are gone consistently).
status: addressed
---
