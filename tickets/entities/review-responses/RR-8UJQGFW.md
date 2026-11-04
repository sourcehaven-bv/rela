---
id: RR-8UJQGFW
type: review-response
title: Scale step blocked submitting an over-scale value
finding: NumberWidget set step=0.01 for scale 2 and the form has no novalidate; a value like 12.345 set stepMismatch and the browser refused to submit although DEC-HWZHA makes a scale violation a warning.
severity: significant
resolution: The step falls back to any while the value has more fraction digits than the scale; widget test added.
status: addressed
---
