---
id: RR-Z14HDK
type: review-response
title: Offers put raw error text on the wire
finding: o.Reason = err.Error() sends any authorize error, including store errors, in every entity GET.
severity: minor
resolution: 'Offer reasons now come from taskItemOfferReason: a ForbiddenError reason or a face error text, otherwise a fixed message, with the error logged.'
status: addressed
---
