---
id: RR-0BMK7CQ
type: review-response
title: Principal names outside the id grammar broke user creation
finding: A t.principal name such as an email address was used as a user entity id, which the id grammar rejects.
severity: significant
resolution: With principal_property the user id is user-<slug>; without it the name is checked by entity.ValidateID and the error says how to fix it.
status: addressed
---
