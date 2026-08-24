---
id: RR-CBM5OP
type: review-response
title: 'opacity 0.85 was tuned as a nudge on top of native greying; as the sole read-only signal it is near-imperceptible'
finding: 'Before this change, `opacity: 0.85` sat on top of the browser''s native disabled rendering — it was a supplement. With `appearance: none` the native greying is gone and 0.85 became the entire read-only signal. A 15% reduction on an accent-filled box is barely visible, so a read-only boolean and an editable one look nearly identical. The comment asserted the muted state was "drawn explicitly rather than inherited" while still carrying the inherited-era value.'
severity: minor
resolution: "Lowered to 0.6 and documented why the sibling widgets' approach does not transfer."
status: addressed
---

The obvious move — copy `background: var(--hover-bg)` from `TextWidget` /
`SelectWidget` / `NumberWidget` — is wrong here, and the reason is worth
recording. On those controls the background is chrome. On a checkbox the
background **is the checked signal**, so overwriting it to convey "read-only"
would erase the state the widget exists to display. Dimming preserves both
axes; a background swap trades one for the other.

0.6 applies to both disabled arms (display and ACL-denied edit).
