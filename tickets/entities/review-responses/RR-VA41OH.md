---
id: RR-VA41OH
type: review-response
title: Range check must cover string and float integers
finding: Integer values arrive as int, int64, integral float64 or numeric string; a check on int alone would miss hand-edited values.
severity: minor
resolution: Normalise all accepted integer forms to int64 before comparing; table test covers each form.
status: addressed
---
