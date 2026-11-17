---
id: RR-EMBHCS9
type: review-response
title: Postgres reads JSON numbers differently from Go
finding: integerExpr cast the numeric text directly, while Go decodes an int64 literal as an int and anything else as the nearest float64.
severity: minor
resolution: integerExpr follows the same two steps in nested CASEs with range guards. TestIntegerExprAgreesWithGo compares it with propmatch.Integer on awkward JSON texts, including underflow to 0.
status: addressed
---
