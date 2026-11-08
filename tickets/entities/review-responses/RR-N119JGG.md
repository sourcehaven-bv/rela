---
id: RR-N119JGG
type: review-response
title: 'Design: peer type must survive principal_property lookup'
finding: After the lookup User is an entity id; if the type claim were lost no baseline would cap the peer.
severity: minor
resolution: 'PrincipalType derives peer from the peer: prefix and Peer() sets the claim, which acl.Resolve copies. Test covers the claim after a rebuild.'
status: addressed
---
