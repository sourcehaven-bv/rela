---
id: RR-VDTW
type: review-response
title: 'TKT-VMD8 AC3 leak-surface enumeration incomplete: Link rel=next, Cache-Control/Vary, timing, X-Page'
finding: 'AC3 covers data.length, meta.total, meta.has_more, X-Total-Count, Link rel=last. Missing: (1) Link rel=''next'' — present iff there''s a visible page after; existence of hidden pages MUST NOT cause next to appear when no visible next page exists; (2) Cache-Control / Vary — list responses are now per-principal; if anything downstream (CDN, browser cache, debug proxy) caches by URL alone, principal A''s filtered response leaks to principal B; require Cache-Control: private, no-store on every ACL''d list response or at minimum Vary on the principal-carrying header; (3) Response timing under DenyAll — if DenyAll short-circuits before search/filter it''s O(1) while type-with-zero-visible goes through GraphCount (one DB roundtrip); accept and document OR normalize; (4) X-Page, X-Per-Page — trivial but pin them. Widen AC3 to enumerate every header.'
severity: significant
status: open
---
