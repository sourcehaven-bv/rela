---
id: RR-DZ3QMVG
type: review-response
title: FindOrphans error is still ignored
finding: 'Code review: found, _ := svc.tracer.FindOrphans(ctx) returns a clean empty section on error, while a re-load error reports an error issue.'
severity: minor
resolution: A FindOrphans error goes through checkFailed as well.
status: addressed
---
