---
id: RR-H97SY
type: review-response
title: rela.md.flatten raises on nil instead of returning empty string
finding: Scripts iterating over list items calling flatten on item.inlines crash on multi-block items (which have children, not inlines). Returning "" for nil would be more useful.
severity: nit
reason: Behavioural change; defer to a small ergonomic follow-up.
status: deferred
---
