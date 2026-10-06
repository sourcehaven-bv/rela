---
id: RR-DMFKP1
type: review-response
title: Byte cleanup errors are silently dropped
finding: DeleteAttachment errors after a stamp or copy were discarded without a trace.
severity: minor
resolution: Non-NotFound errors are logged with slog.Warn naming entity, property and file. The next sweep of the entity collects the bytes.
status: addressed
---
