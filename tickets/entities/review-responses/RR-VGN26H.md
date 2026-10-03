---
id: RR-VGN26H
type: review-response
title: acl-ceiling rule misses visibility and graph-query builders
finding: Globs covered only internal/acl and aclmap; a runtime deny added in internal/visibility or a store query builder would not load the rule.
severity: significant
resolution: Added internal/visibility/** and the pgstore/sqlitestore graphquery files; the root table states the no-runtime-deny invariant.
status: addressed
---
