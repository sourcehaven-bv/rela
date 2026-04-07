---
id: RR-8CNS
type: review-response
title: 'S7: requireLocalHost did not normalise case on r.Host'
finding: Host headers are case-insensitive per RFC 3986, but the allowlist lookup compared raw r.Host bytes against lowercase entries. Mixed-case Host like LOCALHOST:8080 was rejected.
severity: significant
resolution: newSecurity now lowercases entries when building the allowlist; requireLocalHost lowercases r.Host before lookup. Added TestRequireLocalHost_AllowsCaseInsensitiveHost (mixed case allowed) plus a new spoofed-case case in the rejection test to exercise both directions.
status: addressed
---
