---
id: RR-85NWY3
type: review-response
title: Collapsed-sidebar mode makes a no-icon nav item an invisible but clickable row — flagged as an edge case, given no decision
finding: |-
    The plan's edge-case table lists "Sidebar collapsed + no-icon item — collapsed mode hides `.nav-label`, so a no-icon item collapses to an empty row — verify it is not a dead click target". That is the right observation, but "verify" is not a design decision, and this is the one case where the feature actively degrades usability rather than merely looking different.

    Confirmed in the source: `Sidebar.vue:20-24` hides `.nav-label`, `.nav-section-title` and `.logo` when `.sidebar.collapsed`. An item with no icon and a hidden label renders as a blank 38px-tall strip that is still a `RouterLink` (or a `<button>` for actions) — focusable, clickable, and announced to screen readers with its `aria-label`/text content, while being visually empty.

    So the accessibility story and the visual story disagree: a sighted user sees nothing to click, a keyboard or screen-reader user finds a normal item. That is worse than either consistent outcome.

    The HIG citation motivating this ticket is about *menus*, where every item has a label. It does not license a state where the item has neither icon nor label.
resolution: |-
  Addressed in plan (Approach §4a). Collapsed mode falls back to the kind-derived glyph, since `none` means "needs no glyph to be told apart from its labelled siblings" and collapsing removes the labels. The collapsed state lives at the call site, not inside `hasIcon`. Test row added.
severity: minor
status: addressed
---

## Recommended fix

Decide it in the plan rather than deferring to implementation. Two defensible
answers:

**A (recommended) — collapsed mode always shows an icon.** When the sidebar is
collapsed, a `none` item falls back to its kind-derived glyph. Rationale: `none`
expresses "this row does not need a glyph *to be distinguished from its labelled
siblings*". Collapsed mode removes the labels, so the premise is gone and the
icon becomes the only affordance. This keeps every row clickable-looking and is
a two-line template change.

**B — hide the item entirely when collapsed.** Consistent, but silently removes
a navigation target, which is worse.

Whichever is chosen, it needs a test: mount `Sidebar` with `collapsed` and a
`none` item, and assert the row is not simultaneously empty and interactive.

## Related

This interacts with RR-D8I2R2's shared `hasIcon` helper — under option A the
sidebar's decision becomes `hasIcon(x) && !collapsed ? … `, so the helper must
not bake in the collapsed case. Worth resolving them together.
