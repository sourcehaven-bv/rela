---
id: RR-BK9RQ2
type: review-response
title: Truncation can split a UTF-8 rune
finding: Cutting the decoded path at byte 512 could land mid-rune; slog then escapes the value and perf-report shows the escapes.
severity: minor
resolution: truncateUTF8 backs off to a rune start (test TruncateUTF8_KeepsRunesWhole).
status: addressed
---
