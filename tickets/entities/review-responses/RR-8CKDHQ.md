---
id: RR-8CKDHQ
type: review-response
title: 'Code-review minors: cell escaping, per-build timeout, mintID O(n²), CRLF'
finding: 'Cranky-review minors: (a) mdCell pipe-escaping was applied to description cells but not typeName/enum-label/matrix-type cells; (b) buildTimeout was applied per-island via applyTimeout, giving no effective global ceiling (N islands × 30s); (c) mintID did a full O(n) ListEntities scan (each cloning every entity) per create → O(n²) seeding; (d) CRLF input left stray \r in island bodies handed to Lua.'
severity: minor
resolution: '(a) mdCell now applied to typeName, enum value/label, and the matrix type cell. (b) Build wraps the ctx in a single context.WithTimeout(buildTimeout) child so the whole build is bounded (an earlier caller deadline still wins). (c) mintID uses a per-type seedCounts counter. (d) parse normalizes CRLF→LF up front. Nits left as-is (documented behavior): depth=0→default, a ```rela fence with trailing text renders literally, mixed explicit/auto id collision errors loudly.'
status: addressed
---
