---
id: RR-PXMQZPG
type: review-response
title: Two new Go lines exceed the 120-character limit
finding: file.go:543 (149 chars) and sql.go:277 (126 chars) fail lll, so the lint job fails.
severity: significant
resolution: Wrapped both literals; just lint clean on the changed packages.
status: addressed
---
