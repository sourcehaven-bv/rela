---
id: RR-HQ7JZI
type: review-response
title: Revocation e2e changes realtime.yaml, not acl.yaml as AC 21 says
finding: 'AC 21 reads ''removing read access in acl.yaml makes the channel disappear for the second user''; channels.spec.ts:128-140 narrows the channel `read` rule in realtime.yaml instead, because acl.yaml is read once at start (realtime-project.ts:7-9). R23 also asked for per-test helpers on the `api` fixture; the spec writes files from a fixture override for the whole file. Both choices are defensible, but the AC and the test disagree. Fix: update AC 21 to name the realtime.yaml read rule, or restart the server with a changed acl.yaml in that test. The final `await bob.expectNoPanel()` (line 139) passes trivially because the panel is already gone; drop it or make it assert something new.'
severity: minor
resolution: Confirmed rela-server does not watch acl.yaml, so the spec keeps realtime.yaml, AC 21 in the plan now names the realtime.yaml read rule with the reason, the spec comment says why, and the trivial final expectNoPanel was dropped.
status: addressed
---
