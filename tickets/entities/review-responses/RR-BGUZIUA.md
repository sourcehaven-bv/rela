---
id: RR-BGUZIUA
type: review-response
title: Cached local times can hide a concurrent local edit
finding: r.localTimes caches the local updated time per run and is keyed by id without a face. A local edit during the run is judged with the older time.
severity: minor
reason: A run is short and the next run judges again with a fresh time; faced rows are not synced (push refuses faces). Revisit if faced sync is added.
status: deferred
---
