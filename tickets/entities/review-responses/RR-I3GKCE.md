---
id: RR-I3GKCE
type: review-response
title: 'Security: redirect response is cacheable'
finding: 'Security review: the 308 carries no Cache-Control, so a shared or browser cache may keep a principal-dependent redirect and serve it after the reader''s access changes.'
severity: minor
resolution: 'Now 307 with Cache-Control: no-store; asserted in tests.'
status: addressed
---
