---
id: RR-EMH0AC
type: review-response
title: EndpointMatch semantics per selection mode are unspecified
finding: Ruling D5 says evaluate a faced endpoint at the query's face selection but the design does not say what that means for AtFaces when the endpoint type lacks those faces or for AllFaces. graphquerynaive and both SQL builders must agree.
severity: significant
resolution: 'Amendment A6: InWorld resolves by the endpoint type''s chain; AtFaces tests only those faces (no match when absent); AllFaces matches on any endpoint row. RunGraphDifferential covers each mode in PR 8.'
status: addressed
---

## Finding

Ruling D5 says evaluate a faced endpoint at the query's face selection but the
design does not say what that means for AtFaces when the endpoint type lacks
those faces or for AllFaces. graphquerynaive and both SQL builders must agree.

Design: `.ignored/stage2-design.md` section 11.
