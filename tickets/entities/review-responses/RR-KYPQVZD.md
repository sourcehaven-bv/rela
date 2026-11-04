---
id: RR-KYPQVZD
type: review-response
title: Inbox placement relied on an unstated invariant
finding: Moving the inbox to decisionSeq assumes a shown message implies session.cause.
severity: minor
resolution: Plan 6 states the invariant (the server renders the waiting message by its cause) and adds a test for an entry with a message and cause 0.
status: addressed
---
