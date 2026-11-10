---
id: RR-7BYPVLL
type: review-response
title: 'Code review: tenant show without a control table'
finding: tenant show on a database with no control table prints the raw relation-does-not-exist error.
severity: nit
reason: The error is accurate and names the missing table; show is an operator diagnostic, and server start already reports this case with a pointer to tenant create.
status: wont-fix
---
