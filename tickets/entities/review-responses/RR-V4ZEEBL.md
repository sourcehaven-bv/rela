---
id: RR-V4ZEEBL
type: review-response
title: Form and header matching gaps
finding: ParseForm read bodies only for POST/PUT/PATCH; multipart temp files were not removed; first-value-only and the unmatched Host/Content-Length headers were undocumented.
severity: minor
resolution: Urlencoded bodies are parsed with url.ParseQuery for any method; multipart forms call RemoveAll. The header and repeated-value limits are documented. Multipart file parts stay unmatchable (form takes field values).
status: addressed
---
