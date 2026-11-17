---
id: RR-00P8LF2
type: review-response
title: Size cap checked only after preflight and slot
finding: The upload route refuses an oversized body by Content-Length before work; attach_file only hit the cap inside storeUpload.
severity: minor
resolution: Length checked against maxAttachmentBytes before the preflight and the upload slot.
status: addressed
---
