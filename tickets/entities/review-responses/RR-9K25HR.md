---
id: RR-9K25HR
type: review-response
title: running field comment should name the cancel grace
finding: The comment said postgres waits as a side effect of closing its pool, without the bound.
severity: nit
resolution: Comment now says it waits only up to neoq's cancel grace period.
status: addressed
---
