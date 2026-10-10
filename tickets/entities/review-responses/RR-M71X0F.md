---
id: RR-M71X0F
type: review-response
title: Automation push failure log carries the pile name
finding: '[security] internal/autocascade/piles.go logs the interpolated pile name (and it may interpolate entity field values) on failure. Pile names are user data and are never logged elsewhere.'
severity: minor
resolution: autocascade push-failure log carries only automation, entity ref and error; pile name and owner removed. TestRunner_FailedPushLogOmitsPileAndOwner.
status: addressed
---
