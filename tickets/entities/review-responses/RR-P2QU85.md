---
id: RR-P2QU85
type: review-response
title: 'Undefined behaviour: how a span row wraps, and what a partially-filled row does'
finding: |-
    The plan flags 'spans that don't sum to 12 (e.g. two span: 5) -- remainder behaviour must be defined and documented' as an edge case, but never defines it. That leaves a design decision for implementation time, which is what design review is supposed to eliminate.

    With `grid-template-columns: repeat(12, 1fr)` and `grid-column: span N`, CSS Grid auto-placement already answers these -- the question is whether its answer is the one we want:

    1. Two `span: 5` fields: they occupy columns 1-5 and 6-10, leaving 11-12 empty. The row does NOT stretch to fill. This is Filament's behaviour and is almost certainly right, but it must be stated so nobody 'fixes' it later.

    2. Overflow: `span: 8` followed by `span: 6` -- the second does not fit in the remaining 4 columns, so grid pushes it to a new row, leaving 4 columns of the first row empty. Sensible, but surprising to an author who assumed source order maps to visual order.

    3. Narrow viewports: the plan says spans 'must collapse to full width rather than crush' but sets no breakpoint. At what width does a 3-across row become stacked? A `span: 4` at 400px is ~110px -- unusable for a date input. This needs a concrete rule (e.g. below some container width, all items become span 12), and it interacts with FEAT-M448/FEAT-U9GY2 which the ticket declares out of scope.

    4. Hidden fields: if a field is filtered out before render (ACL, empty, conditional), does its span leave a hole? A `span: 4` row where the middle field is ACL-hidden currently reflows the third field into the gap, changing the visual grouping based on WHO is looking. Worth deciding deliberately -- it is arguably a subtle information leak about what was withheld.

    Item 4 is the one to think hardest about: the ticket already commits to preserving ACL behaviour.
severity: significant
resolution: 'Row behaviour is now specified in the ticket rather than deferred to implementation, and is AC 10. (1) Unfilled remainder stays empty — no stretch — matching CSS Grid''s natural behaviour and Filament''s, documented so it is not later ''fixed''. (2) A field that does not fit the remaining columns wraps to the next row, leaving the remainder empty. (3) Below a defined container width all items collapse to span 12. (4) The ACL concern is resolved by evidence rather than mitigation: git-crypt inaccessible fields are RENDERED as an InaccessibleField lock placeholder, not removed (sectionEditFields.ts:96), so they hold their grid cell and nothing reflows. `visible:`-redacted properties are dropped from the wire, but CLAUDE.md is explicit that field-level redaction hides values only and makes no claim to conceal which properties exist — the metamodel is served over the API — so a resulting gap leaks nothing that is not already public. No layout-based oracle exists.'
status: addressed
---

Item 4 connects to the ticket's own AC 8 (ACL behaviour unchanged) and to the
project-wide row-gating rule in CLAUDE.md: a hidden entity should be
indistinguishable from a nonexistent one. A reflowing grid gap is a weak but
real signal that *something* was removed.
