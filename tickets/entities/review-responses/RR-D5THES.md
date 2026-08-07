---
id: RR-D5THES
type: review-response
title: 29 of 40 radius substitutions silently changed rendered values; commit claims visual equivalence
finding: |-
    The commit message and ticket present this as a value-substitution PR with unchanged visual output. That is false for most of the radius migration.

    Counted from the diff (`git show 9066f140 -- frontend/src | grep '^-  border-radius'`):

    | Was | Count | Now | Delta |
    |---|---|---|---|
    | 4px | 13 | --radius-sm 4px | 0 (ok) |
    | 6px | 11 | --radius-md 6px | 0 (ok) |
    | 8px | 10 | --radius-md 6px | **-2px** |
    | 3px | 3 | --radius-sm 4px | **+1px** |
    | 2px | 1 | --radius-sm 4px | **+2px** |
    | 16px | 1 | --radius-lg 12px | **-4px** |
    | 12px | 1 | --radius-lg 12px | 0 (ok) |

    So 15 of 40 declarations changed value, and the 8px->6px group hits nine card-ish surfaces at once (SidePanel .panel-section, EntityList .list-content/.mobile-card, DashboardView .dashboard-card/.validation-card, KanbanView .kanban-filters/.kanban-column/warning banner/.swimlane-grid). Individually invisible; collectively a systematic reduction in corner softening across every card in the app.

    Two are worse than cosmetic:
    - EntityList .filter-chip 16px -> 12px on a 4px/10px-padding chip: was near-pill, now visibly boxier.
    - EntityList .sort-order 8px -> 6px on a ~12px-tall, font-size:9px badge: a 2px delta is ~17% of the element height; it was nearly a pill and now has a visible corner.

    Note scales.css's own comment claims '10/12/16/20 collapse to -lg' and '2/3/4 collapse to -sm' -- 8px appears in NEITHER list, yet it was absorbed into -md. The documented mapping does not match the implemented one.

    The risk is not the pixels; it is that reviewers of PR 2 and PR 3 will diff screenshots against a baseline they were told was unchanged, and a future bisect of 'when did the cards get pointier' lands on a commit asserting nothing changed.
severity: significant
resolution: |-
    Fixed in 27bc6ded by making the claim true rather than by documenting the deltas. The radius ramp gained a fourth step so tokens match values actually in the tree: --radius-sm 4px, --radius-md 6px, --radius-lg 8px (the card radius, 10 uses), --radius-xl 12px. 2px/3px/16px/10px/20px are now deliberately NOT mapped — no token holds those values, so they stay literals rather than being rounded.

    Reverted the component files to develop and re-ran the migration from a clean base with a value-preserving map (the first attempt at remapping in place desynced, because a positional walk over the original values was thrown off by intervening `50%` declarations — reverting was the correct recovery).

    A verification script now compares every migrated declaration against its develop value by resolving the token: 126 radius/font-size declarations preserved, 0 changed; 72 gap/box-shadow declarations preserved, 0 changed. The 'visuals unchanged' claim is now accurate, so the PR 2 / PR 3 screenshot baseline is trustworthy.

    Also corrected the scales.css comment, which had documented a mapping ('2/3/4 -> -sm, 10/12/16/20 -> -lg') that did not match the implementation and omitted 8px entirely.
status: addressed
---
