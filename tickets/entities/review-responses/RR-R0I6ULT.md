---
id: RR-R0I6ULT
type: review-response
title: 'Small cleanups: fixture comment, helper location, string value, mailto target'
finding: The fixture comment was stale, the href helper lived in the widget, the widget did its own string coercion, and a mailto link opened a blank tab.
severity: nit
resolution: Updated the comment, moved the helper to utils/linkHref.ts, used useStringValue, and dropped target for mailto.
status: addressed
---
