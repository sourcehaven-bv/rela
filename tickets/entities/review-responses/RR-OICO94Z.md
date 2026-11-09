---
id: RR-OICO94Z
type: review-response
title: Page cursor at MaxRangeBound cannot advance
finding: checkLast accepted a last of 4096 bytes; the next bound Last+NUL+1 exceeds the store's bound.
severity: minor
resolution: checkLast requires len < MaxRangeBound; enumerate fails clearly on such a local address. TestDiff_LongCursorIsRefused.
status: addressed
---
