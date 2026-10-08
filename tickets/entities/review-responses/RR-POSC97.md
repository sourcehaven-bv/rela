---
id: RR-POSC97
type: review-response
title: Entity-route subtests do not exercise PermitsRead
finding: handleV1GetEntity resolves through visibility.Resolver (ReadableFacesMany directly); only the comments route reaches PermitsRead, but the doc implied both pinned it.
severity: significant
resolution: 'Doc comment now states the comments route is the PermitsRead regression test and the entity route pins the resolver. Verified by mutating PermitsRead to default-world semantics: only the comments granted-face case fails.'
status: addressed
---
