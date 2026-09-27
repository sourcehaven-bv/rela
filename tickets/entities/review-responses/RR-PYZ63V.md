---
id: RR-PYZ63V
type: review-response
title: Verdict slice length is unchecked in applyViewCondition
finding: applyViewCondition indexed the verdict slice without checking its length against the candidates.
severity: nit
resolution: Length guard returns an error.
status: addressed
---
