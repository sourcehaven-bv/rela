---
id: RR-MZU4
type: review-response
title: ETag is principal-independent — denied principals can read cached bodies
finding: 'computeEntityETag (api_v1.go:1969) hashes (ID, Type, Content, Properties, edges) only — no principal. After this PR, GET on a visible entity returns 200 + ETag X; the same URL for a denied principal returns 404 but the ETag stays the same. Two enumeration paths: (a) shared HTTP cache (CDN/proxy/browser) serves cached 200 body to the denied principal via If-None-Match revalidation that returns 304; (b) ETag values observable in proxy logs / cache state become an existence oracle. Fix: a denied response must NOT emit ETag and must NOT honor If-None-Match (always return 404, never 304). Either bake the principal into the ETag or set Cache-Control: private, no-store on per-entity responses when ACL is active. Add tests asserting no ETag header on denied GET and that If-None-Match=<alice-etag> from Bob returns 404 (not 304).'
severity: critical
resolution: 'Incorporated into rescoped TKT-VQGN scope: denied per-entity GET suppresses ETag, sets Cache-Control: private, no-store, ignores If-None-Match (always 404 never 304). Pinned by AC5.'
status: addressed
---
