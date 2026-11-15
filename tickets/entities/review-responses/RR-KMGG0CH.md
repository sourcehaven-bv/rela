---
id: RR-KMGG0CH
type: review-response
title: Opening fence still uses TrimSpace
finding: The opening fence check still trims leading whitespace.
severity: nit
reason: Nothing is inside YAML yet when the opener is checked, so an indented opener cannot be block-scalar text; tightening it could only break hand-written files.
status: wont-fix
---
