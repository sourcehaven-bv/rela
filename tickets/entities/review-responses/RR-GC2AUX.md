---
id: RR-GC2AUX
type: review-response
title: Field validator is not reusable as is
finding: validateConversionFields takes a ConversionDef and the pattern matches only source|new.
severity: minor
resolution: 'Plan: refactor the helpers to take fields and fromType; explicit namespace allowlist.'
status: addressed
---
