---
id: RR-7LMA
type: review-response
title: Control-only header value bypasses fall-through
finding: sanitizeUser ran strings.TrimSpace BEFORE the control-char replacement. A header value like "\x00\x00\x00" survives TrimSpace (NULs aren't whitespace), becomes "   " after substitution, and is returned as a non-empty user string. ChainResolvers short-circuits on it — audit log attributes edits to whitespace instead of falling through to "unknown".
severity: critical
resolution: 'Reordered sanitizeUser to replace control chars + length-cap in a single pass, then trim. Control-only payloads now sanitize to "" and the chain falls through. Added regression test TestHeaderPrincipalResolver_Sanitizes/control-only_payload_sanitizes_to_empty. Files: internal/dataentry/router.go:198-220, internal/dataentry/principal_test.go:177-186.'
status: addressed
---
