---
id: RR-A5LJ7K
type: review-response
title: Attachment operationId can collide with a camelCase type
finding: Type taskAttachment's get collides with type task's attachment download.
severity: nit
reason: Every generated operationId (get<Type>, list<Plural>) shares this risk; types are kebab-case in practice. TestSpecIsStructurallySound catches a collision in its fixture.
status: wont-fix
---
