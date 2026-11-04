---
id: RR-IHWNWQQ
type: review-response
title: follow_up_error text may carry other entities' data
finding: Automation and cascade error text can name entities the caller cannot read.
severity: nit
resolution: The response carries only the code follow_up_failed; the error text goes to the server log.
status: addressed
---
