---
id: RR-FY6Z4V
type: review-response
title: created-another handler leaves a promise unhandled
finding: '@created-another binds an async function directly; it relies on linkToAnchor never rejecting.'
severity: nit
resolution: created-another goes through onCreatedAnother, which voids the promise explicitly.
reason: ""
status: addressed
---
