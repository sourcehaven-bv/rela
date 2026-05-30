---
id: RR-019AK
type: review-response
title: No frontend widget-name constants
finding: Widget identifiers appear as bare strings throughout registry.ts, FieldRenderer.vue (isCheckbox/isRrue equality), tests. Go side has WidgetCheckbox='checkbox' constants. One typo silently falls through to default. Not blocking.
severity: nit
reason: Nit. Real but cosmetic. Worth doing as part of the Go<->TS shared types story if/when one materialises; not worth blocking this PR on. Tests already pin the canonical names by importing TextWidget etc. by component reference, so a typo in a widget-name string would surface as 'widget renders text instead of expected widget' rather than going undetected.
status: deferred
---
