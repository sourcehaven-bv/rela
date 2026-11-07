---
id: RR-4VLZYEE
type: review-response
title: Result message carries the subject and an IdP-supplied status
finding: '[security] clear() put uid and tostring(user.status) in the message, while the receiver keeps the subject out of its own log line.'
severity: nit
resolution: clear() messages no longer contain the uid; the status is shown only when it is inactive or pending, otherwise "unknown". The pre-existing created/updated messages are unchanged.
status: addressed
---
