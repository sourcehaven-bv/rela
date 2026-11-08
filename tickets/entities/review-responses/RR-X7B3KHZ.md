---
id: RR-X7B3KHZ
type: review-response
title: Credential edge cases
finding: Proxy from environment on plain http; any header name accepted; Retry-After HTTP-date form; token file permissions.
severity: minor
resolution: Credentials come only from a named file or variable; empty or unset is an error; values are trimmed and must be printable ASCII; header names are validated and reserved headers refused; a Bearer prefix is not doubled; the token redacts itself in fmt.
status: addressed
---
