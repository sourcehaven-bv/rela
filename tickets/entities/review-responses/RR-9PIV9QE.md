---
id: RR-9PIV9QE
type: review-response
title: Conflict resolve on postgres writes an unread file
finding: On pgstore the resolved file is not the source of truth. The script comment should say the file only drives the handler.
severity: minor
resolution: The script comment says the store is postgres and the resolved file only drives the handler.
status: addressed
---
