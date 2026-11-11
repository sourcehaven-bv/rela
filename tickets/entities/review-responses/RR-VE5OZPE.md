---
id: RR-VE5OZPE
type: review-response
title: File downloads are not rate limited
finding: OpenFile had no limiter, so a reader could stream files without bound.
severity: minor
resolution: OpenFile takes the read limiter with op file-read at FileReadRate (120 per minute per reader and channel). Tested in TestOpenFile_RateLimited.
status: addressed
---
