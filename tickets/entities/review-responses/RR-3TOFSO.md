---
id: RR-3TOFSO
type: review-response
title: Index derivation source and EXPLAIN target
finding: listScopeIndexProperties derives no traversal specs and covers only list-named scopes. The query actually issued is MatchingIDs with e.id = ANY(ids); the EXPLAIN must target that shape. Refusing symmetric relations may drop an existing derived index.
severity: minor
resolution: StaticIndexSpecs now derives traversal index specs from every declared query scope (scopeTraversalSpecs), not only list-named ones. The pg EXPLAIN test also runs the MatchingIDs SQL for a 50-id page and asserts no seq scan; the plan uses the derived index. No related() over a symmetric relation exists in any in-tree config.
status: addressed
---
