---
id: RR-MBLOQI
type: review-response
title: PUT on a closing stream returns 204
finding: 'Code review: a PUT landing after the stream closed but before unregister returns 204.'
severity: nit
reason: 'Harmless: unregister drops the subscriptions moments later, and the client gets the next ready either way.'
status: wont-fix
---
