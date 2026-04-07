---
id: RR-GUXG
type: review-response
title: Error response needs correlation ID for debugging
finding: Generic 'action failed' toast with no correlation ID means users have nothing to grep in logs when reporting issues. Add UUID per request.
severity: significant
resolution: 'Generate UUID per request, included in error response body and server log. Frontend shows ''Action failed (ref: xxx)'' in toast.'
status: addressed
---
