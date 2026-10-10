---
id: RR-SDTLPK
type: review-response
title: 409 reconcile detail still carries the cause
finding: reconcileDetail appended cause=%q and passed raw err.Error() for other errors.
severity: minor
resolution: reconcileDetail now carries only request-derived fields and is empty for other errors; the 500 path logs the cause.
status: addressed
---
