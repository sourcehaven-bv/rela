---
id: RR-G6Z0Y
type: review-response
title: DocumentsPanel returnTo ignores selectedDoc
finding: 'frontend/src/components/entity/DocumentsPanel.vue:85 passes returnTo=`/entity/${type}/${id}` — drops the currently-selected document in the panel. User picks doc ''release_notes'', clicks edit, submits → lands on entity page with whatever default doc is shown. scrollBehavior can''t help because the target HTML is in a different tab. Fix: either put selectedDoc in the route (?doc=release_notes query param) and include in returnTo, or change scope and accept limitation. Former is the right fix and makes panel state shareable/bookmarkable.'
severity: significant
resolution: 'DocumentsPanel now seeds selectedDoc from route.query.doc on mount (falls back to first available), and keeps ?doc=<name> in sync via router.replace as the user switches tabs. Both sides of the route (initial + after-submit redirect) carry the doc identity, so submit returns to the same tab. returnTo includes ?doc=<name>. Verified in browser: URL now reads /entity/applicatie/X?doc=applicatie_overview and edit links carry return_to=%2Fentity%2Fapplicatie%2FX%3Fdoc%3Dapplicatie_overview.'
status: addressed
---
