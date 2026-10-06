---
id: RR-SYJ30H
type: review-response
title: Extra 412 round trip after a merged relations save
finding: relationsBase never moves after a merged relations save so every later relations save costs a 412/GET/resend and uses up attempts.
severity: minor
reason: 'Correct as is: the extra round trip only costs latency. Fixing it needs the form to adopt merged relation lists, which the relation widgets do not support today.'
status: deferred
---
