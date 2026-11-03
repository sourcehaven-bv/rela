---
id: RR-H3L0W5
type: review-response
title: 'Ordered comparison on a nil-bound foreign property fails at render'
finding: 'A property only another collected type declares compiled with that type''s type and bound nil, so `entity.due < today()` passed rela validate and failed every render that collected a type without `due`.'
severity: critical
resolution: 'Foreign properties are now declared as predicate.NilType in CompileForCollection: only ==/~= compile on them, an ordered comparison or date function is a load error. A `where:` on `type` narrows the compiled types first (TraverseConditionTypes). Tests: TestEvaluator_CompileForCollection, TestCompileViewConditions_TraverseWhereNarrowsTypes; docs updated.'
status: addressed
---
