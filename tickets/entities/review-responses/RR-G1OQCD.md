---
id: RR-G1OQCD
type: review-response
title: Failure gives no root cause and needs a database
finding: Without the fix the test only reports a timeout.
severity: minor
resolution: Added TestListenerConnConfig_StripsPoolKeys (no database) for URL and key/value DSNs; it also checks a real server parameter survives.
status: addressed
---
