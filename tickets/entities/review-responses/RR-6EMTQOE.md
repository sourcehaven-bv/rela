---
id: RR-6EMTQOE
type: review-response
title: Stub fails with a bare not found when the script is missing
finding: If scripts/<hook> is absent or not executable the stub gives an unhelpful error.
severity: nit
resolution: The stub checks -x and prints which script is missing and that --no-verify skips it.
status: addressed
---
