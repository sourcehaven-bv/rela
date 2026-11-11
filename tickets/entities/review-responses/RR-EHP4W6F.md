---
id: RR-EHP4W6F
type: review-response
title: AC3 expects 404 but read denial is 403
finding: open returns 403 read_denied for a missing read role; 404 only for an unreadable anchor.
severity: minor
resolution: AC3 now says the same response as reading the channel; OpenFile refuses files of properties no longer declared file.
status: addressed
---
