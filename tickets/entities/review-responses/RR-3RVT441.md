---
id: RR-3RVT441
type: review-response
title: 'Design: gate closure CTEs cost the whole type'
finding: The ACL gate's recursive closure seeds from every entity of the type, so a small leaf range under an inherited-role gate still walks the whole closure. The statement-count budget cannot see it.
severity: significant
resolution: 'TKT-VP02WHG bounds how often a peer can make the server pay the per-type gate cost: a token bucket per principal (2/s, burst 20), 4 server slots and 2 per principal, a 15 s deadline answering not_run, and caps of 32 ranges and 256 type names per request (reconcile_gate.go, reconcile_handler.go). Seeding the closure from the range ids stays an optimisation for when range sizes are measured.'
reason: 'Recorded as a known limit in the plan and the godoc: the cost is one statement whose work grows with the gated type, the same as the search and list gates today. Seeding the closure from the range ids is an optimisation for step 5 once real range sizes are measured.'
status: addressed
---
