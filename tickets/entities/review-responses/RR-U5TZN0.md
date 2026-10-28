---
id: RR-U5TZN0
type: review-response
title: SPA shows the git:sync message for every 403
finding: ConflictsView.conflictError mapped any 403 to the git:sync message, including resolve's write-check 403 and path_outside_project, sending users after a permission they already hold.
severity: significant
resolution: writeGitSyncForbidden now uses the permission_required problem type. ConflictsView shows the git:sync message only for that code; other 403s keep the server message and Retry. ConflictsView.test.ts pins both.
status: addressed
---
