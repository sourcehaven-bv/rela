---
id: RR-RSSNPUP
type: review-response
title: actor:list accepted an unknown type
finding: list of a misspelled type returned an empty list, which a case could read as no access.
severity: significant
resolution: list raises when the schema declares no such type.
status: addressed
---
