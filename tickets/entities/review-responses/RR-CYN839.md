---
id: RR-CYN839
type: review-response
title: Formats differ per destination; write errors silent
finding: stderr lines carry time/level, syslog lines do not; slog drops writer errors.
severity: nit
resolution: 'Formats documented in the destination table. No write-error warning: it would go to the same journal that is failing.'
status: addressed
---
