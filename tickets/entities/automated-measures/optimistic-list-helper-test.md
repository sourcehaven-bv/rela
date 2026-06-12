---
id: optimistic-list-helper-test
type: automated-measure
title: 'Unit tests: shared optimistic-list cache helper (three orderings)'
description: 'Vitest suite (frontend/src/queries/optimisticList.test.ts) covering beginOptimistic/beginOptimisticRemove/rollbackOptimistic/settleOptimistic against a fake QueryCache: copy-on-write update + remove, and the three RR-IVBO9K orderings — success (settle invalidates), error-with-no-refetch (rollback restores), error-after-intervening-refetch (rollback skipped via the identity check). Plus entities.test.ts for the canonicalListParams collision/order fix.'
kind: test
location: frontend/src/queries/optimisticList.test.ts
status: active
---
