---
id: RR-I4LNTE
type: review-response
title: No test that a faced row gets its own face's count
finding: All handler tests used faceless entities, so dropping Face from the target would pass.
severity: significant
resolution: 'Fixed. TestCommentCounts_PerFace lists POL-1 under two world rankings and expects the count of the served face. Mutation check: dropping Face fails it.'
status: addressed
---
