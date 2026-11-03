---
id: RR-YL7PK19
type: review-response
title: Error detail misdirects rela-desktop users
finding: rela-desktop serves the same detail but has no --jwt-* or --principal-header flags.
severity: minor
resolution: 'The detail now reads ''no identity: local $RELA_DATAENTRY_USER, or rela-server --jwt-*/--principal-header; see docs/server-security.md'', which applies to both binaries.'
status: addressed
---
