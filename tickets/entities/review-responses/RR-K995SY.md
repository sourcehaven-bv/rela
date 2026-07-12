---
id: RR-K995SY
type: review-response
title: CLI sync client aborts whole push run on the new 409 create-conflict
finding: The no-upsert rework makes the sync server return 409 on a create-intent conflict (concurrent first-create, postgres multi-writer). The CLI pushResult mapped only 200/412/422; a 409 fell to default -> statusError -> Push returns the error, ABORTING the entire push run and discarding topo-ordered progress, instead of halting just that record like 412 does.
severity: significant
resolution: Added case http.StatusConflict in pushResult returning PushResult{Conflict:true, CreatedConcurrently:true}, flowing through the same classifyConflict -> OutcomeConflict path as 412 (loop continues, per-record index preserved for re-run). Added a dedicated CreatedConcurrently flag for a distinct message ('created concurrently by a peer... resolve with rela sync push/pull --force'). New client test TestPush_CreateConflict409_HaltsOneRecordAndRunContinues. Fixed in commit c95947da.
status: addressed
---
