---
id: RR-WCNOIZ
type: review-response
title: Empty per-type list renders empty rows
finding: A type with no columns gets rows with no cells in a flat table.
severity: minor
resolution: Empty per-type lists are refused, and columns_by_type needs a columns fallback unless it covers every type.
status: addressed
---
