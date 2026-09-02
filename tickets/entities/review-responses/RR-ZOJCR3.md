---
id: RR-ZOJCR3
type: review-response
title: settings_handlers.go comment left with an unfinished rewrap
finding: The edited line now runs past the file's prevailing width
severity: nit
resolution: Rewrapped the comment to the file's prevailing width.
status: addressed
---

The comment edit in `internal/dataentry/settings_handlers.go:84-85` replaced the
bracketed type reference but did not rewrap, leaving a line noticeably longer
than its neighbours. Cosmetic, but it is in the diff.
