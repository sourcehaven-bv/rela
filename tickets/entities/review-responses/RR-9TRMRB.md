---
id: RR-9TRMRB
type: review-response
title: Cancelled run reported as an action failure
finding: A cancelled ctx during the script was logged as a warning and returned as an ActionError.
severity: minor
resolution: run now detects ctx cancellation, skips the warning and sets the cause, so actionError returns the context error.
status: addressed
---
