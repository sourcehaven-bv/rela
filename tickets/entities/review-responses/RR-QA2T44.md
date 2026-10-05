---
id: RR-QA2T44
type: review-response
title: acl can parses the address twice
finding: runNoPolicy calls ParseRef, swallows its error, then requireAddressExists parses again.
severity: nit
reason: The first parse only detects a face address; requireAddressExists owns the invalid-address message, so parsing once would duplicate that message in two places.
status: wont-fix
---
