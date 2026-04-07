---
id: RR-RR8M
type: review-response
title: 'C1: Vue SPA still issues GET to /api/command — POST-only handler breaks command execution'
finding: frontend/src/components/entity/CommandModal.vue line 37 calls `await fetch(url)` (GET) and the bundled v2/assets file shipped the same pattern. internal/dataentry/static/app.js line 245 also called `fetch(url).then(...)` with no method. After making handleCommandExec POST-only, the Vue SPA could not run any command — it would always 405. The dead helper executeCommand in frontend/src/api/commands.ts still defined a GET-only EventSource path that, while unused, would have been the obvious next regression.
severity: critical
resolution: 'Updated CommandModal.vue:37 to `fetch(url, { method: ''POST'' })`. Updated internal/dataentry/static/app.js:245 to POST. Removed the dead executeCommand from commands.ts so EventSource cannot creep back in. Rebuilt the Vite bundle (npm run build) so the embedded v2/assets matches source. Verified by inspection of the new bundle.'
status: addressed
---
