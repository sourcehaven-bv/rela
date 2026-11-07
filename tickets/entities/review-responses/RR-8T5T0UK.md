---
id: RR-8T5T0UK
type: review-response
title: 'Design: budget test must call the reference directly'
finding: memstore delegating to elementnaive over itself bypasses the Counting wrapper.
severity: minor
resolution: 'Plan changed: the budget test calls elementnaive with a Counting-wrapped store.'
status: addressed
---
