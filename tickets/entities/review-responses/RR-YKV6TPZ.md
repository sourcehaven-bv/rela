---
id: RR-YKV6TPZ
type: review-response
title: Canvas probe to read alpha
finding: backgroundAlpha draws the colour on a canvas to read its alpha.
severity: nit
reason: It reads any colour syntax the browser returns (also color() and oklch); a regex on rgba would not.
status: wont-fix
---
