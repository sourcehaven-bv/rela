---
id: RR-SYMIN
type: review-response
title: Symmetric self-inverse relation is routed as incoming because presence-in-map is used as the direction test
finding: '"routePrefilledCardRelations (DynamicForm.vue:1041-1053) builds canonicalByInverse then tests `const canonical = canonicalByInverse.get(key); const suffix = canonical ? INCOMING_SUFFIX : OUTGOING_SUFFIX`. For a `symmetric: true` relation whose inverse.id equals its own name — explicitly legal and documented as the one permitted name overlap (internal/metamodel/loader.go:602) — getInverseName(''related'') returns ''related'', so the map holds related → related and the OUTGOING group key is routed to `related-incoming`. It happens to recover because buildRelationsPatch resolves the inverse of ''related'' back to ''related'', so the body key is right by luck. The test for ''is this key an inverse'' must be `canonical !== key`, not `canonical != null`."'
severity: critical
resolution: The direction test is now `mapped !== undefined && mapped !== key`. Extracted the decision into a pure planPrefillRouting (prefillRouting.ts) so it is testable directly — the first version of the test passed against the buggy code because the body key recovers by luck, which was the reviewer's point about tautological tests. The extracted test fails under mutation.
status: addressed
---

## Suggested resolution

Change the direction test to `canonical !== undefined && canonical !== key`. Add a unit test covering a symmetric self-inverse relation.
