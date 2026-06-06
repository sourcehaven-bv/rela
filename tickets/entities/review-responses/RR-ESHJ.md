---
id: RR-ESHJ
type: review-response
title: Conditional-request headers beyond If-None-Match need explicit deny-path rule
finding: 'AC5 covers ETag/If-None-Match. Current code doesn''t emit Last-Modified or honor If-Modified-Since, so today there''s no risk — but the plan''s per-entity gate sets the precedent. The next maintainer who adds Last-Modified for SPA cache-friendliness won''t know they need a deny-side suppression. Pin in GUIDE-acl-security: ''All conditional-request headers (If-None-Match, If-Modified-Since, If-Match, If-Unmodified-Since, If-Range) MUST short-circuit on deny BEFORE consulting underlying entity state.'' Add a test asserting this for every conditional header the handler emits, even if today''s set is just If-None-Match.'
severity: minor
status: open
---
