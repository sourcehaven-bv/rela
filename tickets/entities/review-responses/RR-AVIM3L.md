---
id: RR-AVIM3L
type: review-response
title: Docs overstate what the listing hash proves
finding: The listing hash describes current bytes and is cached on fsstore by size and mtime; only the audit record is evidence. Also raised by the security reviewer.
severity: minor
resolution: The attachment security guide now says to compare the listing hash with the attach-render audit record and describes the fsstore cache caveat.
status: addressed
---
