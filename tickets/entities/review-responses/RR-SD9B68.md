---
id: RR-SD9B68
type: review-response
title: Duplicated slog body in writeRelationsApplyError
finding: The relation-write 500 copied the log call because writeInternalError could not take a detail.
severity: minor
resolution: Added writeInternalErrorDetail; writeInternalError delegates to it.
status: addressed
---
