---
id: RR-4R3J2XM
type: review-response
title: 'Design: declarative webhooks /hooks/<id> have no tenant'
finding: POST /hooks/<id> is unauthenticated and carries no org, so in tenant mode it is denied without notice.
severity: significant
resolution: '/hooks/ is a tenant prefix: refused without a verified org. Each tenant app with webhooks configured logs a warning at open.'
status: addressed
---
