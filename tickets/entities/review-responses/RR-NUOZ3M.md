---
id: RR-NUOZ3M
type: review-response
title: Shared mutable package var for the OpenAPI minimum
finding: 'internal/openapi/channels.go:489 adds `var zero = 0.0` and shares `&zero` across every generated schema. A generic package-level name and a shared pointer that any spec post-processing could mutate. The same file already writes `Minimum: new(float64)` at line 380; use that.'
severity: nit
resolution: 'Replaced the shared var zero with Minimum: new(float64), as elsewhere in the file.'
status: addressed
---
