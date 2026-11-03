---
id: RR-G03VSY
type: review-response
title: Replay panic crashes the server
finding: 'Security and code review: replayHash ran handler.ServeHTTP on the worker with no recover. net/http recovers a handler panic per request; a replay panic would end the process and recur on every restart while the subscription is re-sent.'
severity: significant
resolution: replayHash defers a recover that logs and reports the replay as unfinished. Pinned by TestLiveHub_PanickingReplayRecovered.
status: addressed
---
