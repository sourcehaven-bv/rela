---
id: RR-SXET9IM
type: review-response
title: Non-ASCII names are not Unicode-normalized
finding: A pasted NFD link to a name such as categorieën does not match an NFC schema key, so the filter is ignored.
severity: nit
reason: 'Outside this bug: the server compares names byte for byte too. Normalize on both sides if it ever matters.'
status: deferred
---
