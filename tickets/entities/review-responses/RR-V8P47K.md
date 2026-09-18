---
id: RR-V8P47K
type: review-response
title: Stylesheet link had no error path and no load ordering
finding: ensureStylesInjected appends a <link> and never observes it. If it 404s (most likely on a checkout
  where the frontend build has not run, so the embedded asset is empty) the editor mounts fully functional
  and completely unstyled with no console message at all. The negative-test list covered the editor failing
  to CONSTRUCT but not the other silent-degradation path.
severity: minor
resolution: 'Added a link error handler that reports the failure and names the likely cause, while deliberately
  leaving the editor working: an app that can still capture text is better than one that cannot. Load
  ordering is left as-is; ProseMirror re-measures on layout and no test could observe the gap without
  asserting on a frame boundary.'
status: addressed
---
