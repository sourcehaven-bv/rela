---
id: RR-YL7XW2
type: review-response
title: e2e upload helper counts failed uploads
finding: submitAndExpectCreateWithUploads counted every PUT response, including failures.
severity: nit
resolution: Only ok responses count; a failed upload rejects at once.
status: addressed
---
