---
id: RR-ZQPQNQE
type: review-response
title: Bearer godoc names the wrong JWT default header
finding: Bearer said Authorization is the JWT gate's default; it is X-Auth-Assertion.
severity: nit
resolution: Removed the claim from the godoc.
status: addressed
---
