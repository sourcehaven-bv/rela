---
id: RR-CBTICK9
type: review-response
title: 'Checkmark sat low and right in the box — inherited position from RelationCards was never centred'
finding: 'Reported by the user on visual inspection of the running demo: the tick "feels slightly off", sitting low-right with dead space along the top and the vertex nearly touching the bottom edge. Confirmed by measurement — the painted ink left 2.20px clearance on the left but only 1.20px on the right of the 14px inner box. The position came from RelationCards.vue''s `.inline-edit-checkbox` (`left: 5px; top: 1px`) and was copied verbatim along with the arm geometry, so the same misalignment has been shipping in the inline relation editor.'
severity: minor
resolution: 'Moved to `left: 4px; top: -1px` (2px left, 2px up), tuned by eye at 13x against the alternatives in a purpose-built static comparison page, then verified on the real widget in the running app. The arm geometry (5x10, 2px stroke) is unchanged — it matches the native glyph well; only the position moved.'
status: addressed
---

Two wrong turns on the way, both recorded because the reasoning is what the
comment in the SFC now guards against.

**1. Deriving the position instead of looking at it.** The first attempt solved
for the geometric centre of the rotated ink bounding box (4.5px/0.94px, giving
1.7px clearance on all four sides) and shrank the arms to a tidier 2.4:1 ratio.
Overlaid on a real `appearance: auto` control at 16x it was visibly worse:
further from the native glyph AND undershooting its arm length. Reverted.

**2. Testing the wrong property.** That overlay compared our glyph against the
*native* glyph, which answers "do these two match" — not the actual question,
"does the tick sit well inside its own box". Native's tick is itself off-centre,
so matching it inherited the defect. Judging the widget in isolation, which is
what the user's screenshot did, makes it obvious immediately.

The shipped value is deliberately NOT the arithmetic centre. A tick's visual
mass is in its long upper-right arm while the eye tracks the lower-left vertex,
so the bounding-box centre reads as low. The SFC comment states this and warns
against "fixing" it back to the computed value — without that, the next reader
recomputes the centre and reintroduces the bug.

`RelationCards.vue` still carries the original position. Not changed here for
the same reason as [[RR-CBLEV8]]: it is a working component on a surface this
ticket's acceptance criteria never exercise. The shared-class follow-up should
adopt the corrected position.
