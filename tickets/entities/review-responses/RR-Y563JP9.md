---
id: RR-Y563JP9
type: review-response
title: Y-axis tick labels were rounded
finding: Compact notation with one decimal showed 0.25 as 0.3 and 0.75 as 0.8, so labels did not match their gridlines.
severity: significant
resolution: New tickFormatter takes its decimals from the tick step and uses compact notation only from 10000 up; unit test covers niceTicks(0.4, 0.9) and a compact axis.
status: addressed
---
