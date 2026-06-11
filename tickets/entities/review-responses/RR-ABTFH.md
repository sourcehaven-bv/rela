---
id: RR-ABTFH
type: review-response
title: Contract drops cross-cutting props (error, disabled, label, optionVerdicts)
finding: WidgetProps declares value/propertyType/mode/onChange/options. Today FieldRenderer also passes error, readonly, optionVerdicts, label/help. Burying these under untyped options blob means every widget hand-rolls casts and the per-widget discriminated union becomes a lie.
severity: critical
resolution: 'Plan revised: cross-cutting concerns lifted to first-class WidgetProps fields (disabled, readonly, required, error, id, placeholder, optionVerdicts). options remains for genuinely per-widget config only. See TKT-MZSIJ ''Widget component shape''.'
status: addressed
---
