---
id: RR-JP0KZ2
type: review-response
title: Conflict routes accept any conflicted file under the project root
finding: Detail and resolve accept any project file with conflict markers, not only files under entities/ or relations/. After this change only git:sync holders reach them.
severity: minor
reason: Outside this bug's scope; only git:sync holders reach the routes now. Limiting resolveConflictPath to EntitiesDir and RelationsDir can follow separately.
status: deferred
---
