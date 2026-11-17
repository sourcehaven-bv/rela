---
id: RR-FV5CTS8
type: review-response
title: Differential values miss integer edge cases
finding: graphDiffValues had no integer-shaped strings and no values at the int64 edge, so OrderInteger disagreements there would pass the differential test.
severity: significant
resolution: Added integer-shaped strings, the int64 bounds as strings, the string 1e19, 4.0 and -2.5 to the value mix and the int64 bounds to the targets. A real beyond int64 stays out of the shared mix because sqlite's text form of it differs under OrderText (TKT-4I0XFKO); TestIntegerExprAgreesWithGo covers it on postgres.
status: addressed
---
