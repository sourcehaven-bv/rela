---
id: RR-58ENT7
type: review-response
title: pg visible search shape has no EXPLAIN pin
finding: Moving the visibility clause inside DISTINCT ON evaluates the ACL EXISTS chain per candidate face row; no EXPLAIN or budget test pins the new plan.
severity: minor
reason: Design item A8 moves the EXPLAIN tests to Stage 3 PR 11; this shape is listed there. The clause must be inside DISTINCT ON for correctness.
status: deferred
---
