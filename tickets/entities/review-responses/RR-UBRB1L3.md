---
id: RR-UBRB1L3
type: review-response
title: Skipping a record when the request context ended loses records
finding: The client may have received the whole body before the context ended. Recording too little reopens the bug.
severity: minor
resolution: recordServed records regardless of the request context.
status: addressed
---
