---
id: RR-EFZ7VB
type: review-response
title: 'E2E: the precedent exists, and a fixture-wide realtime.yaml has side effects'
finding: '§10 says the fixture "may not have two-user auth". e2e/tests/presence.spec.ts already opens two contexts with `X-Forwarded-User` (ALICE and BOB). Putting realtime.yaml in the shared fixture project would mount ChannelsPanel and two more live keys on every entity page in every other spec. Fix: follow presence.spec.ts, and write realtime.yaml (and the acl.yaml roles for the no-post-role user) per test through a typed helper.'
severity: minor
resolution: E2E follows presence.spec.ts with two X-Forwarded-User contexts and writes realtime.yaml and acl.yaml per test through a typed api helper (R23).
status: addressed
---
