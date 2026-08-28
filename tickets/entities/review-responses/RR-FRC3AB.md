---
id: RR-FRC3AB
type: review-response
title: 'The opaque ring abutted an accent border on 20 of 23 sites — a solid 3px blob with no inner contrast'
finding: 'Nearly every swept rule reads `input:focus { border-color: var(--accent-color); box-shadow: 0 0 0 2px var(--focus-ring) }`, and `--focus-ring` IS `--accent-color`. So a 1px accent border and a 2px accent ring sat flush: one undifferentiated 3px band, with the ring at 1:1 contrast against the thing it was ringing. The 4.14:1 / 5.85:1 figures used to justify full opacity were measured against the PAGE BACKGROUND — the outer edge. WCAG 2.2 §1.4.11 requires 3:1 against adjacent colours, plural, and the inner adjacency fails. Counted: 20 sites set both.'
severity: significant
resolution: 'Added a third token, `--focus-ring-gap` (= `--bg-color`), used as the inner of two shadows: `0 0 0 2px var(--focus-ring-gap), 0 0 0 4px var(--focus-ring)`. Applied to all 23 ring sites. Compared the options rendered side by side at real size before choosing — a background separator band beat both the status quo and the alternative of dropping the focus border-color, because it keeps the border cue AND gives the ring its own shape.'
status: addressed
---

The reviewer's sharpest point: **this was already solved once and not
generalised.** `CheckboxWidget` had picked up a two-shadow rule earlier in the
same session, for the extreme version of exactly this problem — a CHECKED
checkbox is *filled* with the accent, so an accent ring around it is invisible
(1:1), meaning focused+checked looked identical to unfocused. That insight
applied to the other 22 sites and had not been carried across. It is now a
token rather than a local trick, so the next ring gets it for free.

Verified in the running app across all four states — unfocused, focused,
error+focused, and checked+focused — in both themes.
