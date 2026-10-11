---
id: RR-F8UGPT
type: review-response
title: Client disconnects logged and answered as 500
finding: writeInternalError did not special-case context.Canceled.
severity: minor
resolution: logInternalError returns false on context.Canceled and the helpers write nothing, like writeGateError. Covered in TestWriteInternalError_HidesTheCause.
status: addressed
---
