---
id: RR-ALBR6PB
type: review-response
title: Peer key in the identity header becomes a logged user name
finding: A client whose --auth-header names the proxy's principal header would send the key as the user name, which then reaches the access and audit logs.
severity: minor
resolution: 'stampAuditPrincipal refuses a resolved user name that is a peer key: 401 on the API, the anonymous default elsewhere, never logged. TestPeerKey_MisplacedInIdentityHeader pins it.'
status: addressed
---
