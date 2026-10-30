---
id: RR-5CFFFQL
type: review-response
title: Search still swallows other filter errors
finding: matchesPropertyFilters still reads any filter error as no match. An unknown enum value or an unsupported operator still gives an empty result without a message.
severity: minor
reason: Separate defect with its own design question (400 or warning for query cards). Filed as BUG-4WKAMFF.
status: deferred
---
