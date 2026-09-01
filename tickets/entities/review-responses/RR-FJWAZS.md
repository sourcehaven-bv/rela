---
id: RR-FJWAZS
type: review-response
title: Drill builds and folds the entire forest before discarding all but one subtree
finding: 'handleV1Gantt calls buildGanttForest unconditionally: every source entity listed/redacted and every hierarchy relation listed on every request, including ?root= drills that then keep one subtree. MaxNodes bounds the response, never the work — O(all entities + all relations) per drill click on a large project.'
severity: significant
reason: 'Deferred with cause: a subtree-scoped build needs an id-set-scoped GATED list seam that does not exist (the scoped lister is per-type wholesale; per-id gating via getVisible is the N-load amplification RR-4JF5U3 warns about). After RR-C3HONH the SPA only server-drills when the tree was truncated, so the expensive path is the rare one; kanban/calendar already do whole-type fetches per view. Revisit when a real project demonstrates the cost — the fix belongs together with a principal-keyed cache design, since the fold is per-principal.'
status: deferred
---
