---
id: RR-ZTD5AZ
type: review-response
title: No HTTP test for a guarded restore
finding: The HTTP restore tests used an unguarded machine, so the ForbiddenError to 403 mapping on restore was unpinned end to end.
severity: minor
resolution: 'Added TestHistoryRestore_PastEntryStateNeedsGuard: 403 and no row without the guard, 200 with it.'
status: addressed
---
