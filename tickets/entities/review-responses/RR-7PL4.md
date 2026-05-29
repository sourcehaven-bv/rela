---
id: RR-7PL4
type: review-response
title: 'Dry-run endpoint shape: ?dry_run query param vs Prefer header — pick one, document caching'
finding: 'Plan lists both ?dry_run=true and a Prefer header as equivalent. Pick one for the wire contract (per api-reference verb rules). Recommendation: ?dry_run=true query param — simpler for the SPA axios client, visible in logs, no header-parsing. Also specify: dry-run responses must be Cache-Control: no-store (they''re value-dependent and per-request) and must NOT set an ETag (no entity to version). Document in api-reference.md alongside the existing create verb.'
severity: minor
resolution: 'Plan: ?dry_run=true query param (not header). Response Cache-Control: no-store, no ETag. Documented in api-reference.md scope.'
status: addressed
---
