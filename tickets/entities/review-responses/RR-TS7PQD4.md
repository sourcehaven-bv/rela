---
id: RR-TS7PQD4
type: review-response
title: Integer conversion rejected 3.0 and 1e3
finding: Exporters write whole numbers as floats.
severity: nit
resolution: coerce accepts a float that is a whole number up to 2^53 (TestCoerceInteger).
status: addressed
---
