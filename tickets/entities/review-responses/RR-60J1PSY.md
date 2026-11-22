---
id: RR-60J1PSY
type: review-response
title: 'Direct URLs to the flow views still get a 404'
finding: 'On a project without flows, opening the flow tasks, runs or run view by URL still calls its own _flows route and gets a 404.'
severity: nit
reason: 'The sidebar no longer links to these views on such a project, so only a typed or bookmarked URL reaches them, and the views already show their not-found or error state. The report was about the call on every page.'
status: wont-fix
---
