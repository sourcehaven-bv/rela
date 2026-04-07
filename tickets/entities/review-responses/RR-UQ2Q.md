---
id: RR-UQ2Q
type: review-response
title: 'S9: log lines used raw header values — log injection risk'
finding: 'reject() logged Host/Origin/path via fmt-style printing without escaping. An attacker-controlled Origin like `https://evil.example\nsecurity: blocked rule=admin_login_success\n` would inject fake log lines into structured destinations (SIEM, journald, etc).'
severity: minor
resolution: All three header values now go through strconv.Quote, which escapes newlines, control characters, and backslashes. Truncation still happens before quoting to keep lines bounded. Not directly a security vulnerability but a log integrity issue worth fixing.
status: addressed
---
