---
id: RR-CIT6N1T
type: review-response
title: 'Design: no upgrade path for legacy schema names'
finding: 'Schemas named in tenants.yaml never match org_<hash>; their audit directories are keyed by the old name; dsn: and erase: true tenants have no equivalent.'
severity: critical
resolution: 'tenant create -adopt takes over an existing schema; the guide documents show, ALTER SCHEMA RENAME, audit directory rename and create -adopt. dsn: overrides are out of scope (future decorator). RefuseLegacyMap refuses to start while tenants.yaml exists.'
status: addressed
---
