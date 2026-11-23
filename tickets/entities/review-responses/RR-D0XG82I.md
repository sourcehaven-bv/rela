---
id: RR-D0XG82I
type: review-response
title: Capped test assertion could be sharper
finding: The SearchView capped test asserts the text differs from the plain count rather than matching the exact string.
severity: nit
reason: The assertion already distinguishes the capped text from the plain count; pinning the exact locale string adds churn without catching more.
status: wont-fix
---
