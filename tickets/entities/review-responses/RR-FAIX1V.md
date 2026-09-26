---
id: RR-FAIX1V
type: review-response
title: Write errors echo raw store errors to remote callers
finding: writeError's default branch and the delete path returned err.Error(), which can name paths or tables and the gc hint.
severity: minor
resolution: callerError passes through ForbiddenError, entitymanager.ValidationError and attachment.ErrNoFileToDetach; anything else is logged and answered generically.
status: addressed
---
