---
id: RR-I4WQPX
type: review-response
title: <tr> compromise is a half-feature; justification misstates DOM event behaviour
severity: critical
status: addressed
---

**Finding (C2, critical).** The `<tr>` compromise ships a half-feature on a
justification that is factually wrong about DOM events.

The plan says "cmd-click anywhere on the row is handled by the guard letting the
event through to that anchor". That is not how events work: returning early from
the `<tr>` handler does not route the event to the anchor. The anchor's default
action fires only if the click landed physically inside it. An implementer
following this sentence would guard, test on the title, and wrongly believe
row-wide cmd-click works.

Consequence as specified: plain click navigates from anywhere on the row
(learned behaviour), but cmd-click does nothing on ~6 of 7 cells, with no
feedback. That is the *ambiguous* fail-direction, not a safe one — the user
cannot distinguish "unsupported" from "my modifier key didn't register".

**Resolution — pick one and state the cost:**

(a) **Stretched-link overlay.** Anchor stays in the title `<td>`; `.entity-row {
position: relative }` + `.title-link::after { content:''; position:absolute;
inset:0 }` makes the whole row a link target. Valid HTML, keeps table semantics,
one link per row in the a11y tree. **Cost: blocks text selection in the row, and
every nested control (`select-cell` :995, `delete-btn` :1028) needs
`position:relative; z-index:1`.**

(b) **Title-only, made discoverable.** Keep the title anchor, delete the
incorrect sentence, narrow AC 1 from "on a list row" to "on the row's title
cell", and give the title distinct link affordance so the working target is
findable rather than accidental.

(b) without the discoverability change is the half-feature. Decision required
before implementation.
