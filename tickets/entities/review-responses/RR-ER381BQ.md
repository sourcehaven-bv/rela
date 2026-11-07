---
id: RR-ER381BQ
type: review-response
title: 'Security: step 5 must validate type lists and bound bytes'
finding: Elements runs ReadQueryFor and SearchFieldScope per named type and loads up to MaxRows bodies per call; a peer-supplied type list or many large ranges amplify work.
severity: minor
reason: Type names are now deduplicated. Rejecting undeclared types needs the metamodel and belongs to the step 5 handler (POST /api/v1/_reconcile) together with the rate limit (RR-3RVT441) and a byte or concurrency budget.
status: deferred
---
