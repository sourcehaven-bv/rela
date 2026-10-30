---
id: RR-HO5U1D
type: review-response
title: Warn-only count in docs is now wrong
finding: 'docs-project/entities/guides/GUIDE-realtime.md:1651 says ''Five more checks only warn'', but the list now has six items: the four old ones plus the >10 buttons warning and the label-equals-type warning, which are two separate checks in buttons.go:371 and :384. Fix: say six, or list them without a count.'
severity: nit
resolution: Changed the count to six and listed the two button warnings as separate items.
status: addressed
---
