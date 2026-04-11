---
id: RR-FUIUH
type: review-response
title: Non-string header values silently dropped
finding: Header values that are not strings are silently ignored. A numeric API version header would be silently dropped, sending unauthenticated requests.
severity: significant
resolution: Both parseHTTPRequestOpts and parseConvenienceOpts now raise on non-string header keys/values
status: addressed
---
