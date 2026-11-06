---
id: RR-PX6L3P5
type: review-response
title: Stored roles never expire
finding: A JWT role lasts one assertion lifetime. A stored role lasts until the next webhook for the user so a lost delivery leaves it in force.
severity: minor
reason: A maximum age needs a config surface and an operator resync to be safe to turn on. The guide states that stored roles do not expire and that revocation depends on webhook delivery. Follow-up ticket to add an opt-in max age.
status: deferred
---
