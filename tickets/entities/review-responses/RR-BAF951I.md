---
id: RR-BAF951I
type: review-response
title: 'Security: scoped edge from a new entity skipped the check'
finding: 'affected() drops pairs whose source is blank, so CreateLinked could add a scoped edge from the entity it creates, which has no scope edge, with no check.'
severity: significant
resolution: 'checkScopes refuses a scoped add whose source is the entity being created: it has no scope, so no target is in scope. Covered in TestScoped_CreateLinked.'
status: addressed
---
