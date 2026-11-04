---
id: RR-N4MCN87
type: review-response
title: Decimal notation parser written twice
finding: predicate/value.go and metamodel/decimal.go each parse the notation and declare the 64-character cap.
severity: nit
reason: A shared test table over both parsers is a follow-up.
status: deferred
---
