---
id: RR-P4FYPI
type: review-response
title: CSP-violation e2e test filtered by a hardcoded seed entity ID
finding: The demo app's deliberate startup violations were excluded by matching /api/v1/features/FEAT-001.
  If that probe's target ever changed the test would go red for a reason unrelated to the editor, and
  the obvious repair is to widen the filter — which is how a test that proves something valuable becomes
  one that proves nothing.
severity: minor
resolution: The console listener is attached AFTER waitForEditor resolves, so the window contains only
  the editor interaction. Every violation seen is the editor's by construction, with no filter to widen.
status: addressed
---
