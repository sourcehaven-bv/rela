---
id: RR-CLFLYK
type: review-response
title: opError and actionError near-duplicates
finding: opFailure has two renderers that differ only in the error type.
severity: nit
reason: They render to different domain types (comments.OpError, script.ActionError); the shared part is already in logInternal.
status: wont-fix
---
