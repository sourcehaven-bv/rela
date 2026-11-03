---
id: RR-NYZ2AMB
type: review-response
title: Two lifecycle methods on Local
finding: Local.Stop (timers) and Local.Close (hints) were called at opposite ends of Services.Close.
severity: minor
resolution: Local.Stop now also stops the hint sender (without a flush; the flush ran at retirement). Services.Close calls only Stop. Close(ctx) stays as flush plus stop.
status: addressed
---
