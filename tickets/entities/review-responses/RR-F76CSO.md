---
id: RR-F76CSO
type: review-response
title: Label column is not exactly constant
finding: Label flex-grow 1 against the value's 999 still gives the label a sliver of free space, so the column varies by about 1px between row widths.
severity: nit
resolution: 'Fixed by flex: 0 1 120px (the label no longer grows).'
status: addressed
---
