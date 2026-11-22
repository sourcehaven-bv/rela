---
id: RR-0L8G1Z0
type: review-response
title: 'Stamped identity preferred over the principal'
finding: 'currentUserEntityID preferred a stamped query identity without checking it matched the principal the audit records.'
resolution: 'The value now comes from the principal only; the test pins that a different stamp is not read.'
severity: significant
status: addressed
---
