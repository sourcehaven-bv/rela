---
id: RR-Q0DRRZ
type: review-response
title: Single webhook type shares a namespace with IdP principal_type values
finding: An IdP could issue principal_type webhook; a baseline written for IdP tokens would then satisfy A15.
severity: nit
reason: The ticket specifies `webhook`. A collision only narrows access, and an IdP-issued webhook token is better capped by the webhook baseline than not. Documented.
status: wont-fix
---
