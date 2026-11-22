---
id: RR-H1PIQUS
type: review-response
title: Phone check starts false
finding: 'usePhoneScreen started false and updated only after mount, so the first render on a phone included the hidden rows.'
resolution: The ref now reads matchMedia at once; the listener still tracks changes.
severity: minor
status: addressed
---
