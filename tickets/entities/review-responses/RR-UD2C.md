---
id: RR-UD2C
type: review-response
title: synthDef's list:true for single-value enums is brittle
finding: |
  Every propType-having field is rendered through MultiSelectWidget -- even single-value status. Today this looks like one Badge in a badge-row. Two issues: (a) if field.values is [] (allowed by ViewSectionField), MultiSelectWidget display renders an empty span.badge-row -- no value, no "-" placeholder, no inaccessible affordance. The pre-refactor branch rendered '-' via field.values?.join(', ') || '-'. This is a behaviour regression for present-but-empty enum fields. (b) Long arrays (50+) silently render 50 badges -- no truncation, no overflow handling. Pre-refactor join(', ') at least bounded layout.
severity: significant
status: addressed
resolution: |
  MultiSelectWidget display mode now renders an em-dash ("—") placeholder when the resolved array is empty, restoring the pre-refactor "-" semantic. For arrays longer than the LONG_ARRAY_THRESHOLD (5), it falls back to a comma-joined string instead of rendering N Badges that would break card layouts. Threshold is hardcoded in the widget. Tests cover the empty case, the exactly-at-threshold case, and the over-threshold fallback.
---
