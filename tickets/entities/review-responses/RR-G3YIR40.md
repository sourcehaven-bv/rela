---
id: RR-G3YIR40
type: review-response
title: Conflicted-row skip logged a warning on every query
finding: slog.Warn ran per conflicted row per query so every list logged while a conflict was open.
severity: minor
resolution: The skip logs at debug level. The fsstore error already names the file.
status: addressed
---
