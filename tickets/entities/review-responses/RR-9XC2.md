---
id: RR-9XC2
type: review-response
title: Renumber silently materializes missing values into numeric ordinals
finding: |-
    maybeRenumberSide uses SortRelations and assigns 1.0..N to ALL siblings, including ones that previously had no _order_out. A user who deliberately left a sibling blank (or hand-edited markdown without an order) finds their other files rewritten after a collapse-triggered renumber on an unrelated sibling.

    Contradicts tolerant-storage policy. Fix: skip siblings whose previous value was missing/non-finite; only redistribute among siblings that already had a value. (Or alternatively: emit a warning before bulk fill and document it loudly.)
severity: significant
resolution: 'maybeRenumberSide now redistributes only siblings that previously had a finite numeric value. Missing siblings stay missing; the renumber plan filters them out before assigning ordinals. Added TestUpdateRelation_RenumberPreservesMissing: seeds 3 siblings (1.0, missing, 2.0), forces a collapse, asserts the missing sibling remains missing while the two with values renumber to 1.0 and 2.0.'
status: addressed
---
