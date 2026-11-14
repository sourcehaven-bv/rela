---
id: RR-8SOC5QY
type: review-response
title: Other button dropped when no item has a label
finding: choiceViews skips the question when no option yields a button, also dropping Other.
severity: nit
reason: Matches an empty string list, which also shows no Other; a question with no usable option is not open to an answer.
status: wont-fix
---
