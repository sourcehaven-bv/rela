---
id: RR-299TVAV
type: review-response
title: principal_property lookup can redirect a peer to a user entity
finding: IdentityResolver.Resolve looked up peer:acme against principal_property. Anyone who may write a user entity could set that property to peer:acme and swap the peer's identity and roles.
severity: significant
resolution: 'Resolve skips the lookup for peer: names; the peer name comes from the operator''s key list. TestResolve_PeerIsNeverLookedUp pins it.'
status: addressed
---
