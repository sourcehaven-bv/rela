---
id: RR-7MZ7QF
type: review-response
title: Exported step constructors would skip validation
finding: NewFile plus exported constructors bypass ParseFile, Validate, validateDeltasResolved, validateStepOrder.
severity: significant
resolution: 'Plan changed: an exported spec type is marshalled and parsed through ParseFile, the same round trip Generate does; step types stay unexported.'
status: addressed
---
