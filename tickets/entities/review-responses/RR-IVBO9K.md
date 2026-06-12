---
id: RR-IVBO9K
type: review-response
title: KanbanView optimistic-mutation cache logic has no unit coverage
finding: The copy-on-write onMutate, the getQueryData(key) === optimistic rollback identity check, and onSettled invalidation are only exercised by E2E, which rarely hits the error-with-intervening-refetch ordering.
severity: minor
resolution: 'Closed by TKT-24QVHB: the optimistic-update cache logic was extracted into src/queries/optimisticList.ts (beginOptimistic/beginOptimisticRemove/rollbackOptimistic/settleOptimistic) and unit-tested for the three orderings (success / rollback / refetch-superseded). KanbanView''s moveCard now uses it, and EntityList''s new delete mutation shares it.'
reason: 'Deferred to the EntityList migration slice of FEAT-XY2D1L: the optimistic-update cache logic will be extracted into a shared pure helper in src/queries/ (needed by the second consumer anyway) and unit-tested there for the three orderings (success, rollback, refetch-superseded-rollback).'
status: addressed
---
