---
id: RR-UHNHX4
type: review-response
title: IssuesTable a11y conversion under-specified; mobile variant and palette ARIA missed
finding: IssuesTable's a11y conversion was under-specified and the MOBILE variant was missed — both the desktop cell (:100-109) and the mobile card (:170-178) use the identical role=button + tabindex + keydown.enter + keydown.space.prevent pattern, but the scope table listed only 'entity-title cell', singular. Separately, CommandPaletteModal's <li role=option> sits in a role=listbox driven by aria-activedescendant, so a focusable anchor inside it breaks the roving-focus model.
severity: significant
resolution: Both IssuesTable variants converted to RouterLink with role/tabindex/both keydown handlers removed; entity-less rows (canNavigate false) render a plain span with no role. Space no longer activates, which is correct link semantics and restores scroll-on-space. The palette anchor got tabindex=-1 so it is not a tab stop and aria-activedescendant roving focus is preserved; palette keyboard tests (36) all pass. The unused useRouter import in IssuesTable was removed — with real links the component needs no router at all.
status: addressed
---

**Finding (S5, significant).** `IssuesTable` a11y conversion is under-specified,
and the mobile variant was missed.

Verified: **both** the desktop cell (`:100-109`) and the mobile card
(`:170-178`) use the identical `role="button"` + `tabindex="0"` +
`@keydown.enter` + `@keydown.space.prevent` pattern. The plan's scope table
listed "entity-title cell" — singular. Both must convert or they diverge.

Converting to an anchor requires removing `role="button"`, `tabindex`, and
**both keydown handlers**. Leaving them yields an element announced as a button
but behaving as a link, and Enter double-fires (native activation + handler).

Deliberate behaviour changes to accept: Space no longer activates (correct link
semantics) and page-scroll-on-Space returns, since `@keydown.space.prevent`
currently suppresses it.

`canNavigate()` is false for script-error/load-error rows, which must render a
plain `<span>` with no role/tabindex — the conditional-anchor pattern the plan
already gets right.

**Also:** `CommandPaletteModal.vue:291` is `<li role="option">` inside a
`role="listbox"` driven by `aria-activedescendant`. A focusable anchor inside
`role="option"` violates the listbox content model and can break roving focus —
the anchor needs `tabindex="-1"` at minimum. The plan discussed only the
close-on-modifier-click decision and missed this.
