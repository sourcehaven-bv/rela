---
id: RR-3RVT441
type: review-response
title: 'Design: gate closure CTEs cost the whole type'
finding: The ACL gate's recursive closure seeds from every entity of the type, so a small leaf range under an inherited-role gate still walks the whole closure. The statement-count budget cannot see it.
severity: significant
reason: 'Recorded as a known limit in the plan and the godoc: the cost is one statement whose work grows with the gated type, the same as the search and list gates today. Seeding the closure from the range ids is an optimisation for step 5 once real range sizes are measured.'
status: deferred
---
