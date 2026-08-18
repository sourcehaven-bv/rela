---
id: RR-HUL1JC
type: review-response
title: 'The widget did not snap back to its previous value on decline'
finding: 'Declining left formData untouched - correct - but the widget had already moved its own DOM when the user interacted with it. Because the bound model-value never changed, Vue had nothing to patch back, so the control kept displaying the declined value while the form held the old one. The user sees one thing and the form means another, which is the class of mismatch that killed the four earlier confirm attempts. Every existing test asserted formData and so was blind to it.'
severity: critical
resolution: 'The reject path bumps a render key so the widgets re-read from formData. Deliberately not done by writing a sentinel through formData, which would fire the visibility watcher on a value that was never real. Found independently while writing a DOM-level test; pinned by that test.'
status: addressed
---
