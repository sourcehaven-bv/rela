---
id: RR-G80SDH
type: review-response
title: overflow-x:auto silently makes .kanban-board a vertical clip container
finding: 'Adding overflow-x:auto to .kanban-board coerces overflow-y from visible to auto per CSS spec, making the board a vertical clip container it previously was not — affecting every existing board. Benign today (page scrolls, card hover-shadow fits inside padding) but load-bearing by accident: any future drag ghost, tooltip, popover, or sticky header rendering outside a card box will be clipped. overflow-y:visible cannot opt out; the spec coerces it back.'
severity: significant
resolution: 'Documented the accepted constraint on the .kanban-board rule, matching the comment already on the swimlane rule: overflow-x:auto coerces overflow-y from visible to auto per spec, so the board now clips vertically and overflow-y:visible cannot opt out. The comment records why it is safe today (card hover shadow''s 8px blur is inset by .column-cards'' 12px padding, plus 20px padding-bottom) and names what would be clipped in future (drag ghost, tooltip, popover, sticky column header), so the next person finds the explanation in the right file. No behavior change — the clipping is inherent to scoping the horizontal scroll to the board, which is what the ticket set out to do.'
status: addressed
---

## Finding

`frontend/src/views/KanbanView.vue` `.kanban-board { overflow-x: auto }`.

Per CSS spec a non-`visible` overflow on one axis coerces the other from
`visible` to `auto`. Verified: an element with only `overflow-x: auto` computes
`{x: "auto", y: "auto"}`. So `.kanban-board` now clips vertically, which it did
not before — and this affects every existing board, not just ones with
header/footer.

Practical impact today is benign: `.main-content` sets no height so the page
scrolls rather than the board, and the card `:hover` box-shadow (8px blur)
survives because `.column-cards` padding is 12px and the board has 20px
padding-bottom. But it is load-bearing by accident. Anything later rendering
outside a card's box — drag ghost, tooltip, popover, sticky column header — gets
clipped, and the next person debugs it in the wrong file.

Note `overflow-y: visible` cannot be used to opt out; the spec coerces it back.
The fix is to document the accepted constraint, matching the honesty of the
comment already on the swimlane rule.
