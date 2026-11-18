---
id: RR-F8K95NS
type: review-response
title: Non-scalar field values became nil
finding: A function or userdata field value was silently dropped and then failed as required.
severity: nit
resolution: 'Any non-scalar value raises: field X must be a string, number or boolean. Test added.'
status: addressed
---
