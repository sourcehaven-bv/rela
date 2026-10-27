---
id: RR-T2EB34
type: review-response
title: Docs claim the lint covers Intl formatters
finding: Only toLocale*String calls were checked.
severity: minor
resolution: 'Added selectors for new Intl.X() with no locale or an undefined locale; the guard test probes one. Docs now say new Intl.*Format. Plain Intl.DateTimeFormat() calls stay allowed: the two in the tree are time-zone probes.'
status: addressed
---
