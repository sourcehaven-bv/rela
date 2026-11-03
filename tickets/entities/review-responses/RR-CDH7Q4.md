---
id: RR-CDH7Q4
type: review-response
title: max-width 579px leaves a fractional gap
finding: With fractional layout widths a grid 579.5px wide matched neither condition and a wide tile could add an implicit column.
severity: minor
resolution: 'The query uses range syntax: @container dashboard-grid (width >= 580px).'
status: addressed
---
