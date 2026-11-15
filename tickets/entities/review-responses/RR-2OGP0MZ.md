---
id: RR-2OGP0MZ
type: review-response
title: Parse accepts trailing JSON after the reply
finding: The decoder did not check for a second value after the object.
severity: nit
resolution: Parse refuses a reply with trailing data; tested.
status: addressed
---
