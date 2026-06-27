---
id: RR-FA1G
type: review-response
title: commitImmediately with disabled channels
finding: |
  Current code iterates Object.keys(timers) and fireProperty(p), then checks contentTimer, then relationsDirty. With disablePropertyChannel: true, timers is empty (because scheduleFieldSave throws before scheduling) -- so the loop is naturally empty. Same for content and relations. No code change needed in commitImmediately.
severity: minor
status: addressed
resolution: |
  Don't add explicit "skip-fire" guards in commitImmediately; the existing per-channel state checks already handle it correctly. Add an assertion test that commitImmediately returns { settled: true } immediately on a fully-disabled instance. AC test plan amended.
---
