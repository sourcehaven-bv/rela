---
id: RR-26KB8
type: review-response
title: Typed error / sentinel for custom-ID rejection
finding: fmt.Errorf with a plain string gives callers no way to type-assert. Introduce a sentinel or typed error so the dataentry handler could map it to 400 instead of blanket 422.
severity: nit
reason: Valid suggestion but adds surface area for a single consumer. Defer until a second caller actually wants to branch on this error type. Today the dataentry handler's 422 'validation_failed' mapping is arguably correct -- the caller supplied invalid input that could not be accepted. Promoting to 400 is a cosmetic HTTP-code distinction.
status: deferred
---
