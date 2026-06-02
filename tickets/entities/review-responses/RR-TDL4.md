---
id: RR-TDL4
type: review-response
title: Request listener never removed on failure paths
finding: Line 34 attaches appPage.on('request', ...) and never calls .off(). On the happy path the per-test context.close() cleans it up, but if anything between attach and end-of-test throws, the listener leaks. The page-object helper should wrap with try/finally.
severity: significant
resolution: Listener now lives on the page that the fixture owns end-to-end; context.close() removes it implicitly when the fixture tears down, regardless of whether the test threw. No try/finally needed because the fixture itself is the scope.
status: addressed
---
