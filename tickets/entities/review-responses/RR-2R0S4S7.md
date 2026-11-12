---
id: RR-2R0S4S7
type: review-response
title: Page can land past the end after an action
finding: 'Cranky 8: resolving the last entry on the last page reloads an empty page.'
severity: nit
resolution: RemoteConflictsView steps back to the new last page when a reload comes back empty past the end; unit test added.
status: addressed
---
