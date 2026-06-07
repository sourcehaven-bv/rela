---
id: RR-P2M7
type: review-response
title: /api bare-path bypass (no trailing slash)
finding: 'strings.HasPrefix(r.URL.Path, ''/api/'') misses a bare `/api` request. If a future endpoint mounts at exactly /api (no trailing slash), the middleware silently bypasses ACL. Unlikely today but cheap fix: add `|| r.URL.Path == ''/api''`. Or invert to an allowlist of bypass prefixes (default-deny aligned with fail-loud spirit).'
severity: nit
resolution: 'attachACLRequest now also gates the bare /api path: `!strings.HasPrefix(path, ''/api/'') && path != ''/api''`.'
status: addressed
---
