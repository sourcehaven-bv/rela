---
id: RR-I5OG
type: review-response
title: truncateRunes allocates full []rune(s) for a prefix
finding: Original truncateRunes did []rune(s), materializing the entire decoded string, then copied the first N runes. For a 1 MiB header value capped at 256 runes, that allocated 1 MiB for nothing.
severity: significant
resolution: 'Inlined length-capping into the single-pass sanitize loop using a rune counter. The for-range loop decodes one rune at a time and breaks on n >= principalUserMaxLen. No []rune(s) allocation. File: internal/dataentry/router.go:198-220.'
status: addressed
---
