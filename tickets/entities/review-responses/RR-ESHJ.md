---
id: RR-ESHJ
type: review-response
title: Conditional-request headers beyond If-None-Match need explicit deny-path rule
finding: 'AC5 covers ETag/If-None-Match. Current code doesn''t emit Last-Modified or honor If-Modified-Since, so today there''s no risk — but the plan''s per-entity gate sets the precedent. The next maintainer who adds Last-Modified for SPA cache-friendliness won''t know they need a deny-side suppression. Pin in GUIDE-acl-security: ''All conditional-request headers (If-None-Match, If-Modified-Since, If-Match, If-Unmodified-Since, If-Range) MUST short-circuit on deny BEFORE consulting underlying entity state.'' Add a test asserting this for every conditional header the handler emits, even if today''s set is just If-None-Match.'
severity: minor
reason: 'Today''s code only emits and honors If-None-Match (ETag), and that header is suppressed on deny by TestACLGet_ETagSuppressedOnDeny. The other headers the RR names (Last-Modified, If-Modified-Since, If-Match, If-Unmodified-Since, If-Range) are not emitted or consumed anywhere in handleV1GetEntity. No risk surface exists in this PR. Deferred to TKT-VMD8 because that PR''s list-side handlers will introduce the conditional-header story alongside paging headers (Link, Cache-Control), and the GUIDE-acl-security pin reads more naturally as one combined "all conditional headers short-circuit on deny" rule once there''s a second example to anchor it.'
status: deferred
---
