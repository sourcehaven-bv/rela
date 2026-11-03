---
id: RR-R8T4RQ
type: review-response
title: Busy safety replay is pure cost on single-process backends
finding: 'live_inbox.go:183 puts every busy board with a wake in the safety replay on every backend. On SQLite (DEC-LFSYNY) no other process can open the project, so every channel write passes through this process''s realtime.Local. The safety replay of a busy board can never find a change there, and it still costs one full list pipeline plus ListWaiting per board per 14 s. Fix: gate the busy part of safetyDue on a wiring flag, for example set by SetInboxGrid or a sibling SetCrossProcessChannels from the channel store, that is false for sqlitechannels. Say so in the live_busy.go package comment. Document the TKT-YK30D1 follow-up that can drop it on postgres too.'
severity: minor
reason: The cost is one replay per open busy board per 14 s, the same cost class the inbox safety replay already pays on every backend including SQLite. Gating it needs a process-model flag through sqlchannels (pg and sqlite share one Store type), appbuild, dataentrywire and dataentry. That fits better in a follow-up together with TKT-YK30D1, which can drop the safety replay on postgres too. Follow-up TKT-IV6XUP.
status: wont-fix
---
