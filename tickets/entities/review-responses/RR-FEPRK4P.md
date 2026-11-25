---
id: RR-FEPRK4P
type: review-response
title: Pre-run registration failure is not seen
finding: Before app.Run Wails defers the OS binding and reports a failure later through its error handler
severity: minor
reason: Wails offers no per-shortcut result before Run. The q key and /quick still work and Wails logs the error.
status: deferred
---
