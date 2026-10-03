---
id: RR-93JDCJ
type: review-response
title: Leftover GetEntity names in tests and a caller note in mailtemplate.Build doc
finding: Test doubles kept id parameter names on GetAddress; endpoints_test failure text said GetEntity; mailtemplate.Build doc described its caller.
severity: nit
resolution: Renamed parameters to addr, fixed the message and trimmed the Build doc to its contract.
status: addressed
---
