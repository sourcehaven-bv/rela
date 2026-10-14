---
id: RR-VF4CRI
type: review-response
title: Changing error text bypasses log-once
finding: Errors whose text varies are logged every tick.
severity: minor
reason: Loader errors are file parse and validation messages, stable for an unchanged file; a time throttle would hide a new problem.
status: wont-fix
---
