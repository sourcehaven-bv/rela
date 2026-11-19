---
id: RR-CI30VTN
type: review-response
title: Hint stays after a step when a context source shares the page
finding: refreshType returned one type, so a page with both a context source and a flow_tasks source never re-resolved on run changes; a finished step's hint stayed until the next navigation.
severity: significant
resolution: The response now carries refresh_types with every type (context type and _flow-run); the SPA refreshes on any of them. Table test TestNextActionPage_RefreshTypes and a vitest case.
status: addressed
---
