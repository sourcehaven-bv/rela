---
id: RR-X6KM8HV
type: review-response
title: Purple to accent rule duplicated in styleBarColor
finding: The status-dot purple rule lives in RlStatusDot and here.
severity: nit
reason: The rule is a CSS class in the component library; sharing it would need a new library export for one line. The comment points at the dot.
status: wont-fix
---
