---
id: RR-U0GVB6
type: review-response
title: Budget test missed tone_by and the sort retry
finding: The budget test did not cover tone_by, a related() visible grant, or the table sort retry.
severity: minor
reason: Added a tone_by matrix case (budget 3). The related() visible grant priming is covered by the list endpoint budget tests over the same redactRows path. The sort retry is an error path that reads at most two bounded pages; a budget case needs a policy hiding the sort property on some rows, which the shared fixture lacks.
status: deferred
---
