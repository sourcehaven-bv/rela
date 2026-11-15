---
id: RR-WKIE108
type: review-response
title: Nav status lowered rules before checking an empty plan
finding: 'cranky: navStatusInStore gated every rule''s traversals before the empty check and then discarded the work.'
severity: minor
resolution: 'Addressed: the empty check runs right after pushdownPlan, as the Go path returns before judging rules over no rows.'
status: addressed
---
