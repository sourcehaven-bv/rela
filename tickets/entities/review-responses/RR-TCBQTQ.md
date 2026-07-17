---
id: RR-TCBQTQ
type: review-response
title: 'Dangling-peer status differs by route: 404 (sub-resource PATCH) vs 422 (collection PATCH)'
finding: handleV1UpdateRelation (PATCH /relations/{type}/{targetId}) maps a dangling-peer manager error to 404 relation_not_found, while the collection PATCH maps the same logical condition to 422 target_not_found. Not a security issue (ACL runs first, no ungated write in either), but two routes give different answers for the same cause.
severity: minor
reason: Consistency nit, no security/behavior-correctness impact (both are non-success, no write lands). Deferred to a follow-up that aligns the sub-resource route to 422. Out of scope for the minimal security fix.
status: deferred
---
