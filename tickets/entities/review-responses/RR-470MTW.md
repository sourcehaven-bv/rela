---
id: RR-470MTW
type: review-response
title: No body size limit on the handler
finding: The handler reads the request body without MaxBytesReader; properties accepts arbitrary nested values.
severity: nit
resolution: The handler now wraps the body in http.MaxBytesReader with a 64 KiB limit. Pinned by TestTaskItemHandler_BodyIsBounded.
status: addressed
---
