---
id: RR-IGTZA1
type: review-response
title: Command input 500 switched to JSON
finding: commands.go other errors are plain text.
severity: nit
resolution: Back to http.Error plain text with an slog.ErrorContext line.
status: addressed
---
