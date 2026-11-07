---
id: RR-ER381BQ
type: review-response
title: 'Security: step 5 must validate type lists and bound bytes'
finding: Elements runs ReadQueryFor and SearchFieldScope per named type and loads up to MaxRows bodies per call; a peer-supplied type list or many large ranges amplify work.
severity: minor
resolution: 'TKT-VP02WHG: the handler refuses undeclared and repeated type names with 400 naming the type, caps 256 type names and 2000 listed rows per request, bounds the body to 512 KiB and bound lengths via store.PlanElementRange, and admits requests through a per-principal rate limit and a slot budget (429/503 with Retry-After).'
reason: Type names are now deduplicated. Rejecting undeclared types needs the metamodel and belongs to the step 5 handler (POST /api/v1/_reconcile) together with the rate limit (RR-3RVT441) and a byte or concurrency budget.
status: addressed
---
