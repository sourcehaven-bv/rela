---
id: RR-1A4C66C
type: review-response
title: Roles from another org's webhook are stored
finding: The receiver records OrgID but never compares it. With a shared issuer and audience a webhook for org B could be replayed to org A's server.
severity: minor
reason: Org enforcement is the open R1 design and applies to JWT requests too. The guide now tells operators to give each deployment its own -webhook-audience and notes that the org is not checked.
status: deferred
---
