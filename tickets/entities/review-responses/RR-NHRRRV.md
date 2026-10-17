---
id: RR-NHRRRV
type: review-response
title: IdP receiver identity webhook:<event> is not reserved
finding: 'Pre-existing: /webhooks/idp stamps webhook:<event> without system:, so a principal-header deployment lets a caller assert it.'
severity: significant
reason: 'Pre-existing behaviour of the IdP receiver (/webhooks/idp), which this ticket explicitly leaves out of scope: it is a different mechanism with its own principal. Changing its identity affects existing acl.yaml grants and needs its own migration note. Filed as TKT-RQ9HLJ.'
status: deferred
---
