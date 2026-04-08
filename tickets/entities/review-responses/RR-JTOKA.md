---
id: RR-JTOKA
type: review-response
title: ConfirmModal busy label uses ASCII ellipsis and inline template literal
finding: Busy label renders as 'Delete...' (three dots) instead of the Unicode ellipsis '…'. The expression also uses a template literal in the Vue template, which is readable but slightly awkward. Move to a computed and use '…'.
severity: nit
resolution: Introduced busyConfirmLabel computed in ConfirmModal.vue that returns `${confirmLabel}\u2026` (Unicode ellipsis) when busy, else confirmLabel. Template interpolates {{ busyConfirmLabel }} instead of an inline template literal. Updated the corresponding test to assert the Unicode character.
status: addressed
---
