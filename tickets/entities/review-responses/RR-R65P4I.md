---
id: RR-R65P4I
type: review-response
title: countIsZero applies the default scope to a first-run hint
finding: nextaction.go's countIsZero calls scopedSortedEntities with a nil query, so no ?query_scope= is present and the entity type's default scope applies. The question 'does this user have any entities of this type' is therefore now answered as 'any non-archived entities', so the first-run hint fires for a user whose entities are all archived.
severity: minor
reason: 'Correct as it stands, on the feature''s own terms. countIsZero drives a first-run UX hint — it decides whether to show ''you have nothing yet, here is how to start''. That is a presentation question, which is exactly what a query scope is for, and a user whose every task is archived arguably SHOULD see the empty-state hint on their task screen rather than a blank list with no explanation. This is not an integrity surface: it counts nothing anyone relies on for correctness, unlike analyze_* or validate, which AC6 keeps unscoped and which have one test each proving it. Left as-is with the call site''s comment updated to say the scope applies and why that is intended, rather than changed to opt out of a behaviour it wants.'
status: wont-fix
---

The reviewer flagged this as "defensible either way but unremarked", which was
fair — the comment described the pre-scope semantics. The fix is the comment,
not the behaviour.
