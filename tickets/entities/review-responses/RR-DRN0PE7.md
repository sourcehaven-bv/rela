---
id: RR-DRN0PE7
type: review-response
title: Gated CTE depends on trigramIndexable rejecting an empty needle
finding: visiblesearch.go binds likeArg only when q.Text is set. The CTE relied on trigramIndexable of an empty string being false.
severity: minor
resolution: 'Fixed: the condition also checks q.Text.'
status: addressed
---
