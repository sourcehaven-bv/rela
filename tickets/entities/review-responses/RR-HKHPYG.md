---
id: RR-HKHPYG
type: review-response
title: Relation columns resolved over all visible children, not just emitted rows
finding: 'buildNestedTree assembled rowEntities from parents plus EVERY visible child before the budget loop ran, then called resolveRelationColumns over that set. The node budget bounds the response, not this work: a 1,400-epic project with 200 tasks each would pass ~280,000 ids to RelationQuery.EntityIDs and run a gated header batch over every distinct neighbour, all to render at most 2,000 rows. The code comment claiming the cost is ''constant in the number of rows, which the query-budget test pins'' was doubly wrong -- it was constant in the GRAPH size, and no such test exists.'
severity: significant
resolution: Restructured to select-then-resolve. Extracted planNestedRows, which spends the node budget and returns only the parents and children that will actually be emitted; rowEntities is then built from that plan, so resolveRelationColumns sees at most nestedNodeBudget entities regardless of graph size. The misleading comment was replaced with one stating the reason (selection before resolution) and the concrete number it avoids. The false query-budget-test citation was removed rather than left asserting a guarantee nothing enforces.
status: addressed
---
