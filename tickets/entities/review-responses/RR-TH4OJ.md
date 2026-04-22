---
id: RR-TH4OJ
type: review-response
title: expectCardHasClass string-to-RegExp not anchored
finding: new RegExp('card-added') matches 'uncard-addedxyz'. Only caller uses a regex already; drop string branch or anchor.
severity: nit
reason: Nit. Only caller uses a regex already; the string path is defensive and never exercised. Can be dropped in a future pass.
status: deferred
---
