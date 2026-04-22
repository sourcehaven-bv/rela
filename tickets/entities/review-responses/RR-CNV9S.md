---
id: RR-CNV9S
type: review-response
title: linkTargetByIdWithSearch has 4 positional args — swap-risk
finding: linkTargetByIdWithSearch(widget, targetId, searchText, reason). Swap-args footgun. Use an options object.
severity: minor
reason: linkTargetByIdWithSearch is called from exactly one test. Argument-swap risk is low. Defer to an API cleanup ticket if it gets more call sites.
status: deferred
---
