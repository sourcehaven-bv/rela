---
id: RR-3FC3O
type: review-response
title: Validation layer choice (validateEntitySemantics, accumulating)
finding: 'Reviewer flagged this as a question to close: should validation go in validateEntityStructure (bail on first) or validateEntitySemantics (accumulate)? Plan picks the latter.'
severity: nit
resolution: 'Confirmed: validateEntitySemantics is correct. A bad display_property reference doesn''t poison anything else, so the author should see it alongside any other label/property issues in the same load attempt. validateEntityStructure is reserved for structural integrity (reserved names, conflicting prefixes). Closing for the record.'
status: addressed
---
