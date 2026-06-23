---
id: RR-7TIU
type: review-response
title: filterVisibleIncludes fail-open under store-error — partial results contradict the comment
finding: 'In filterVisibleIncludes (api_v1.go ~1850), the Query branch iterates a.store.GraphQuery(ctx, q) and `break`s on error. But entities yielded BEFORE the error have already been marked `allowed[e.ID] = true`. The comment explicitly says ''Fail-closed on store error: include nothing of this type'' — the implementation does the opposite for any partial-yield scenario (network blip mid-stream, ctx cancel after first row, pgx scan error on the third row). An attacker with a coarse role + a flaky store delivers a denied principal a stream of ''visible'' neighbours the policy actually denies. Worse, the response still looks 200 OK with a partial included map. Fix: build a localAllowed map inside the Query branch; only merge into allowed after the iterator completes cleanly. On err, discard the local map (true fail-closed).'
severity: critical
resolution: filterVisibleIncludes now builds a per-type local map and only merges into allowed after the iterator drains cleanly; on err, drops the whole type (true fail-closed).
status: addressed
---
