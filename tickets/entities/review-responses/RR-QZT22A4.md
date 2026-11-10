---
id: RR-QZT22A4
type: review-response
title: Ctrl-C does not abort the confirmation prompt
finding: signal.NotifyContext is installed before the prompt and ReadString ignores the context, so Ctrl-C at the prompt does nothing.
severity: significant
resolution: Signals are caught only after confirmation, around tenant.Erase.
status: addressed
---
