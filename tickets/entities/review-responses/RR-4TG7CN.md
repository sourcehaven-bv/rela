---
id: RR-4TG7CN
type: review-response
title: Single-hop traversal to an inherit_through-gated type errors in the store
finding: GateTraversal places the far type's ACL HasInbound inside the EndpointMatch (a nested position) and refuses inheritance only when hop.Next != nil. Every backend refuses nested inheritance (CheckEndpointShape), so the list 500s for principals whose read of the far type uses inherit_through. The comment claiming single hops are unaffected is wrong.
severity: significant
resolution: 'gateHop refuses any ACL HasInbound carrying InheritThrough or EntityInheritThrough with ErrTraversalUnsupported regardless of Next, and the comment is corrected. The scope surfaces it as an error, not a 500 from the store. Test: TestGateTraversal_InheritedReadIsRefusedOnASingleHop.'
status: addressed
---
