---
id: RR-A3C4S
type: review-response
title: Inconsistent emit typing across widgets
finding: Text/Textarea/Date/Number/Select emit [value:unknown]. Checkbox [boolean], MultiSelect [string[]], Rrule [string]. Since modelValue is unknown everywhere, the asymmetric narrowing on emit is just noise. v-model consumers see unknown for some and string for others.
severity: minor
resolution: 'Standardised all widget emit signatures to ''update:modelValue'': [value: unknown]. CheckboxWidget, MultiSelectWidget, RruleWidget no longer narrow asymmetrically. v-model consumers see a consistent unknown across the board, matching the WidgetProps.modelValue type.'
status: addressed
---
