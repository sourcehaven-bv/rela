---
id: RR-SAW55H
type: review-response
title: Store and manager decide last face separately
finding: faceDeleteEdges authorizes from its own family count while each backend recounts in DeleteFace. If the counts disagree the store removes inbound edges that were never authorized.
severity: significant
resolution: After tx.DeleteFace the manager checks every DeletedRelations entry against the authorized incoming and outgoing set (requireAuthorizedEdges) and fails the Tx otherwise. Table test in requireauthorized_internal_test.go.
status: addressed
---
