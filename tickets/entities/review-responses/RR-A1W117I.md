---
id: RR-A1W117I
type: review-response
title: min/max value forms and rollup types
finding: extremeFold returns raw int/float/string values; numeric must be judged from the declared type.
severity: minor
resolution: Converted with metamodel.ParseDecimal; numeric judged from the declared property type (rollups included).
status: addressed
---
