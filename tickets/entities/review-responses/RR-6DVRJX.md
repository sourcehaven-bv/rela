---
id: RR-6DVRJX
type: review-response
title: The mockup "deletion" is not a git change, and it breaks the phase-4 mockups
finding: '§2: the fixtures/RlRealtime*.vue, fixtures/realtime.ts and pages/Realtime.stories.ts files are untracked (`??`), so deleting them changes nothing in the PR. RlRealtimeBoard, Session and Entity import fixtures/realtime.ts and stop compiling locally. Fix: say "not committed", and keep realtime.ts (or move the phase-4 types into those files) until phase 4.'
severity: nit
resolution: The plan now says the mockups are not committed, and fixtures/realtime.ts stays until phase 4 so the other mockups keep compiling (R26).
status: addressed
---
