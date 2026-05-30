---
id: RR-CRG00
type: review-response
title: FieldRenderer re-derives widget name as string equality
finding: isCheckbox/isRrue in FieldRenderer re-run defaultWidgetFor and string-compare to 'checkbox'/'rrule'. Second resolution path parallel to registry.resolve(). Label-position decision should be a property of the widget (WidgetEntry.labelPosition), not the consumer's magic-string equality.
severity: minor
reason: Real improvement but it expands the WidgetRegistry contract (adding labelPosition to WidgetEntry) which the design-review specifically flagged as load-bearing for the next four tickets. Worth its own design discussion when TKT-UD7YR (view-side delegation) or TKT-HOIX1 (config surface) actually need to differentiate layout-positions per widget. For this ticket the string-equality magic on resolvedWidgetName is acceptable -- only one widget (checkbox) currently needs labelPosition='after', and the consumer's behaviour is trivially testable today.
status: deferred
---
