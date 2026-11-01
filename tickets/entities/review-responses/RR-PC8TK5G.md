---
id: RR-PC8TK5G
type: review-response
title: Panic after a cancel logs both flags
finding: canceled=true was added whatever completed was, so a panic after a cancel logged status=500 panic=true canceled=true.
severity: minor
resolution: 'Fixed: canceled is only logged for a handler that returned; the panic test case asserts no canceled attribute.'
status: addressed
---
