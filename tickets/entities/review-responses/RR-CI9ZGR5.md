---
id: RR-CI9ZGR5
type: review-response
title: Last 40 lines can miss a failure in an earlier package
finding: With several packages the end of the go test log can be all ok lines, so tail -n 40 may show none of the failure.
severity: significant
resolution: The hook greps the failure lines (--- FAIL, panic, DATA RACE, file:line, build headers) from the whole log and prints the first 60. Verified with a failing calfeed test listed before a passing principal package.
status: addressed
---
