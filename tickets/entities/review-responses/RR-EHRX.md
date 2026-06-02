---
id: RR-EHRX
type: review-response
title: Asset-extension regex misses fragment-only URLs and entangles extension+query logic
finding: /\.(css|woff2?|ttf|eot|svg)(\?|$)/i requires the extension to be followed by ? or end-of-string; URLs like foo.svg#icon would test false. Today fragments don't reach request.url() so this is unreachable, but the same-origin assertion (S3) eliminates the need for the regex entirely. If kept, match against new URL(u).pathname so query/fragment handling is automatic.
severity: significant
resolution: Regex deleted entirely. The fixture-level same-origin check (RR-1FL4) makes per-extension filtering unnecessary — every off-origin URL fails the test, regardless of file extension or query/fragment shape.
status: addressed
---
