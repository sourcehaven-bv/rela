---
id: RR-JW5M3YN
type: review-response
title: Derived tenant DSN weakens TLS
finding: dsnForSchema rebuilt the DSN from a few fields, dropping sslmode=verify-full, root certs and other settings, and did not quote the password.
severity: significant
resolution: 'dsnForSchema now keeps the base string and only sets search_path (URL query or trailing keyword), then re-parses and checks it. Test: TestDSNForSchema_KeepsBaseSettings.'
status: addressed
---
