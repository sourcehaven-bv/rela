---
id: RR-7ZB8
type: review-response
title: 'Request-capture race: passive listener can miss the maxcdn fetch on slow CI'
finding: 'EasyMDE''s autoDownloadFontAwesome code path queues the <link> append in a microtask; Playwright''s request event fires when the browser actually dispatches the GET. expectMarkdownEditorReady() only awaits domcontentloaded + visibility, not network-drain. On a slow CI box the assertion expect(cdnHits).toEqual([]) can run before the queued maxcdn request fires — green when it should be red. Fix: switch to an *active* assertion (throw inside the request handler when a maxcdn URL appears), or wait for networkidle after editor mount.'
severity: critical
resolution: 'Replaced the passive listener-then-assert pattern with a fixture that captures every off-origin request for the lifetime of the page and throws in afterEach (fixtures.ts:341-358). No race window: every request fires through the listener before context.close() runs. The error message names the offending URL so a regression points straight at it.'
status: addressed
---
