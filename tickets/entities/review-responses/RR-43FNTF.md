---
id: RR-43FNTF
type: review-response
title: Bars end one day short
finding: An inclusive end date was drawn at the start of that day; visible at 40px/day.
severity: significant
resolution: Bars, planned inset, overruns, commit marker and past-commit stripe use pos(end + 1); unit test asserts Mon-Fri is 5 x 40px.
status: addressed
---
