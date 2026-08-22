---
id: RR-LISTW
type: review-response
title: "widget on a list property was unvalidated — silent data corruption"
severity: significant
status: addressed
finding: "PLAN-DMQFRJ named the rule ('only multi-select legal on a list property') but the implementation ignored PropertyDef.List entirely. VERIFIED: widget: textarea and widget: text on a {type: string, list: true} property both passed validation. This is not cosmetic. defaultWidgetFor puts list FIRST, above values and type (registry.ts, an order RR-0Z1P6 marks load-bearing), so it is precisely the case where config overrides the SPA's highest-precedence rule. TextareaWidget runs the array through useStringValue and onFieldUpdate then PATCHes the resulting STRING — an operator gets a control that flattens ['a','b'] and writes back a scalar, on a config line the server called valid."
resolution: "Rewrote the check as widgetAcceptsProperty(widget, PropertyDef, meta) — a predicate over the whole PropertyDef rather than a map keyed on Type alone — applying the SPA's precedence: list first, then value-set, then the type table. A non-multi-select widget on a list property is now a config-load error naming the rule. Pinned by TestValidateConfig_WidgetOnListProperty (5 cases)."
---
