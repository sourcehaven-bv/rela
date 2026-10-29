---
id: RR-STBTCQ
type: review-response
title: e2e covers only a never-moved, default-working session
finding: 'e2e/tests/kanban-busy.spec.ts:6-9 relies on the room''s initial state `detected` (e2e/tests/realtime-project.ts:178-182), which has no status and no waiting_for. That is the exact shape the new load check warns about (channels.go:252-255), so the fixture boots with that warning. The spec never moves a session into a working state, which is the PoC flow the feature exists for. Fix: give the e2e session a waiting initial state and a transition into an explicit `status: working` state. Assert the card turns busy after that move and clears after the next one, so no warning is expected at boot.'
severity: minor
resolution: 'Added RoomOptions.run with a RUN session: Queued waits for the owner (entity.owner, added to feature by addBusyBoard), Start moves to Planning (working), Finish ends it; the spec checks not busy when queued, busy with activity line in Planning, and clear after Finish.'
status: addressed
---
