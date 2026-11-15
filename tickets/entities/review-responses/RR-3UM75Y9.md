---
id: RR-3UM75Y9
type: review-response
title: fsstore native header pager makes BodyWatch under-report
finding: fsstore parses each file whole so a native ListEntityHeadersPage reads the same bytes as the fallback; BodyWatch would see a native pager and count zero bodies on the default backend. fsstore does not implement HeaderReader for the same reason.
severity: minor
resolution: Dropped the fsstore method and its plimsoll bump; fsstore is served by the dispatcher fallback and the conformance suite covers it. The HeaderPager godoc records why.
status: addressed
---
