---
id: RR-9SMD
type: review-response
title: int64 in generic constraint is decoration — only float64 used
finding: compareOrdered has int64|float64|string in its constraint but the call site stringifies via %v then ParseFloat, so int64 is never instantiated. Genuine ints lose precision past 2^53. Generic gives illusion of typed comparison without actually using it.
severity: significant
resolution: Generic now uses cmp.Ordered which covers all numeric types properly. The values are still stringified (since map[string]interface{} hides the type from the API layer), but the helper itself is now correct — a future improvement (deferred) is to thread typed property values through from the metamodel.
status: addressed
---
