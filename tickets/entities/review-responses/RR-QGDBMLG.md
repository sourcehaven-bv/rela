---
id: RR-QGDBMLG
type: review-response
title: Failed assembly leaves background workers running after the pool closes
finding: assemble starts soft-delete GC, data-migration GC and mail before finishAssembly\; a later failure did not stop them, and the new closeBackend closes the pool under them.
severity: significant
resolution: assemble defers backgroundServices.stop() when it returns an error.
status: addressed
---
