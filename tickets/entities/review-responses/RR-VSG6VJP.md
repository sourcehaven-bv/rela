---
id: RR-VSG6VJP
type: review-response
title: IdentityBinder must not replace an existing read time
finding: Replacing below the edge splits one operation across two read times.
severity: significant
resolution: IdentityBinder stamps only when ctx has no read time; a long-lived reader stamps per call.
status: addressed
---
