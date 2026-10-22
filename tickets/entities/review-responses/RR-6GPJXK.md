---
id: RR-6GPJXK
type: review-response
title: Repeated principal assertion
finding: The principal check is repeated in two new tests.
severity: nit
reason: Two sites in two packages; a shared helper would need a new shared test dependency. principal.Principal is not comparable, so a struct compare does not work.
status: wont-fix
---
