---
id: RR-KRQ2OCG
type: review-response
title: No runtime cap on the batch request size
finding: The load-time content budget does not bound list values or the encoded message size.
severity: significant
resolution: 'Plan: cap the encoded user message at run time with a named error and cap list lengths.'
status: addressed
---
