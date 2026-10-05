---
id: RR-1D2VI9
type: review-response
title: Lua write gate uses a ListEntities IDs query
finding: writeTargetReadable used rd.ListEntities{IDs; AllStates} in internal/lua; the 8.5 guard (PR 3) forbids that literal there and it loads every face body for a bool.
severity: significant
resolution: 'writeTargetReadable asks Family when the reader provides it (ScriptReader and UnrestrictedReader do). The list read stays only as a fallback for readers without Family: raw stores in tests and dataentry''s lateGatedReader; PR 3 owns that reader and can forward Family and drop the fallback. Both paths pinned by TestWriteTargetReadable.'
status: addressed
---
