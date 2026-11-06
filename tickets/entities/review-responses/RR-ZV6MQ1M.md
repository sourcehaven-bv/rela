---
id: RR-ZV6MQ1M
type: review-response
title: ValidationError godoc overstated the guarantee
finding: The comment said Message never quotes the value but operator-written messages such as a pattern error can.
severity: minor
resolution: The comment now scopes the guarantee to built-in messages and names operator-written messages as the operator's responsibility.
status: addressed
---
