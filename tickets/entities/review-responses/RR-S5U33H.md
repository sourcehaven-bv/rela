---
id: RR-S5U33H
type: review-response
title: The descriptor reload path does not exist yet and the planned hook is costly
finding: '§1 "again on the refresh SSE event … to verify in T7a". Verified: `reloadRealtime` (internal/dataentry/realtime_wiring.go:63) only calls `live.policyChanged()` and never broadcasts `refresh`. The schema store has no reload-on-refresh at all (only Sidebar and useNavEntities listen). Piggybacking on `refresh` invalidates every entity query cache on each realtime.yaml save. Also, if the fetch is added to `doLoad`''s Promise.all, one failing `_schema/channels` request (an older server, or realtime unwired) takes the whole SPA to App.vue''s error screen; the dashboard fetch has an explicit comment about exactly this. Fix: decide now, not "to verify". Load the descriptor lazily in its own small store, swallowing failures to an empty list. Reload it on a dedicated SSE event (`config:realtime`) sent after a successful reload, or when a head names a message type or action that the cached descriptor lacks. Add an AC for that reload.'
severity: significant
resolution: T7a adds a refresh broadcast after a realtime.yaml reload, as Jeroen decided, and the descriptor lives in its own lazily loaded store that falls back to an empty list outside the schema store's Promise.all (R5).
status: addressed
---
