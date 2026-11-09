---
id: RR-0IM7OHE
type: review-response
title: 'Design: peer edge enters the middleware chain at a second point'
finding: Handing a verified request to a handler captured below the JWT gate creates a second entry into the chain; middleware added between them would be skipped for peers.
severity: significant
resolution: 'One identity stage: stampAuditPrincipal verifies a peer key or runs the resolver, and marks a peer request on ctx with an unexported key. requireVerifiedJWT passes a marked request through. The chain is otherwise unchanged and stays inside requestStats.'
status: addressed
---
