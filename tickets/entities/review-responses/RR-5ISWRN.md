---
id: RR-5ISWRN
type: review-response
title: --expect claims complete access but skips rename and fields
finding: '[security] The godoc and help said the file covers complete access; rename and field-level access are not compared, so CI could attest a wider surface.'
severity: minor
resolution: Godoc, --expect help and docs/webhooks.md now name the compared axes and list what is not compared.
status: addressed
---
