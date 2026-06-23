---
id: RR-ENW3J4
type: review-response
title: Fake QueryCache models keys as exact strings, can't catch prefix-vs-exact regressions
finding: optimisticList.test.ts's fake cache uses exact string-join keys, so a regression narrowing settleOptimistic to an exact key (defeating invalidate-all-param-variants) would pass.
severity: minor
reason: The unit under test only forwards keys verbatim to the cache; the exact-vs-prefix contract is owned by Pinia Colada's real cache and exercised end-to-end by the list+kanban E2E specs (create/delete reconcile across pages). Modeling isSubsetOf in the fake would test the fake, not the helper. Left as-is; the E2E layer is the right guard for the prefix contract.
status: wont-fix
---
