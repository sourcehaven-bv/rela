---
id: RR-PBH6L2
type: review-response
title: Header-then-gate pipeline duplicated in batch and endpoints
finding: 'batch.go readableHeaders/singleType/typeGate repeated endpoints.go storedFaces/readableFaces: same header read, mixed-type rule and per-type gate, with drifting details. Two copies of ACL code can diverge.'
severity: significant
resolution: EndpointsReadable now calls readableHeaders; storedFaces, readableFaces and endpointFaces are deleted. The endpoint tests pass unchanged.
status: addressed
---
