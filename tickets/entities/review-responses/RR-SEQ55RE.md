---
id: RR-SEQ55RE
type: review-response
title: No aclstore test for HasOutgoingEdge
finding: The store-backed HasOutgoingEdge had no direct test of the faced tail or the edge condition.
severity: minor
resolution: Added TestGraph_HasOutgoingEdge with faced tail / condition / empty sources / non-source cases.
status: addressed
---
