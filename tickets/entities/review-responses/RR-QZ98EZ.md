---
id: RR-QZ98EZ
type: review-response
title: Delete idempotency contradicts Detach
finding: 'Plan says deleting a missing file succeeds; Detach errors when the property is empty even with a file name. Fix: when file_name is given, delete idempotently; only the no-name form errors on an empty property. Test both.'
severity: minor
resolution: DetachFile deletes a named file idempotently and returns the removed name; an absent file returns "" and both MCP and rela detach say nothing was removed. Unnamed delete still errors on zero or several files. TestService_DetachFile.
status: addressed
---
