---
id: RR-QQR15N
type: review-response
title: Explicit empty status is stored
finding: A create with status "" and no declared default kept the empty string.
severity: minor
resolution: core.go drops an empty status when there is no declared default. Test TestCreate_EmptyStatus.
status: addressed
---
