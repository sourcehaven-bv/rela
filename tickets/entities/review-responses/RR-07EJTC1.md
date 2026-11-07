---
id: RR-07EJTC1
type: review-response
title: Retention sweep blocks CLI exit and ignores the Close context
finding: The sweep ran at start and Close waited for it without honouring ctx.
severity: significant
resolution: The first sweep waits one interval, runs under a cancellable context, and Close selects on ctx.
status: addressed
---
