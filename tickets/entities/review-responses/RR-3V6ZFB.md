---
id: RR-3V6ZFB
type: review-response
title: Guard misses detail variables, other error names and composite literals
finding: TestNoInternalErrorDetail only flagged an identifier named err or a .Error() call in CallExpr args; detail := ..., fmt.Sprint(gerr) and ganttError{500, ...} passed.
severity: significant
resolution: The guard now requires every argument of a call or composite literal naming StatusInternalServerError to be a literal, an http constant or w/r/correlationID. Verified by planting both shapes.
status: addressed
---
