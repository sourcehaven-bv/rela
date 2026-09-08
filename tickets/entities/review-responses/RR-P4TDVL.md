---
id: RR-P4TDVL
type: review-response
title: display:contents on the row anchor removes its link role in WebKit
finding: |-
    `.row-link` used `display: contents`. WebKit drops an element's implicit ARIA
    role under `display: contents`, so the anchor stops being exposed as a link to
    VoiceOver — losing the exact semantics this change exists to add, on the one
    browser where it is hardest to notice. The visual neutralisation this ticket
    needs is already achieved by `color: inherit; text-decoration: none`, so
    `contents` was buying nothing.
severity: significant
resolution: |-
    Changed `.row-link` to `display: inline`. The cell body is inline content in a
    `<td>`, so an inline anchor introduces no layout box worth worrying about; the
    full suite and the visual structure tests stay green.

    `.row-cell` keeps `display: contents` — safe there precisely because that
    element is an inert `<span>` with no role to lose. The CSS comment records the
    distinction so the two are not "made consistent" later.
status: addressed
---

## Resolution

Changed `.row-link` to `display: inline`. The cell body is inline content in a
`<td>`, so an inline anchor introduces no layout box worth worrying about; the
full suite and the visual structure tests stay green.

`.row-cell` keeps `display: contents` — safe there precisely because that
element is an inert `<span>` with no role to lose. The CSS comment records the
distinction so the two are not "made consistent" later.
