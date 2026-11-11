---
id: RR-SQRLC4R
type: review-response
title: Services.Close stops mail and sweeps before the flow worker
finding: stopBackgroundServices stopped mail, gc, soft delete, the version sweep and realtime records before the flow worker, whose writes reach those services.
severity: minor
resolution: The flow worker stops first in stopBackgroundServices.
status: addressed
---
