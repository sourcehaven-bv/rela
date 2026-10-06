---
id: RR-1LWG14
type: review-response
title: ErrAbortHandler and Goexit are logged as panics
finding: completed=false also covers a deliberate panic(http.ErrAbortHandler) and runtime.Goexit.
severity: minor
resolution: 'Documented as out of scope in the requestStats comment: no handler does either, and telling them apart needs a recover that rewrites the stack net/http prints.'
status: addressed
---
