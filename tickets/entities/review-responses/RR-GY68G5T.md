---
id: RR-GY68G5T
type: review-response
title: 401 has no WWW-Authenticate header
finding: RFC 9110 requires WWW-Authenticate on a 401. The existing 401 responses omit it too.
severity: nit
resolution: 'Recorded as deliberate in the helper''s godoc: header mode has no scheme a client could answer.'
status: addressed
---
