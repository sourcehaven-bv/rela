---
id: RR-7AW45P
type: review-response
title: Total replay concurrency unbounded
finding: 'Architecture review: after the PUT-time baseline each PUT had its own semaphore, so N concurrent PUTs ran up to 4N replays beside a round. The type doc still said all replays run on the worker.'
severity: significant
resolution: The hub has one slots semaphore shared by rounds and PUT baselines; a baseline stops waiting for a slot when its PUT is cancelled. TestLiveHub_ReplayLimitShared checks the peak. The doc is corrected.
status: addressed
---
