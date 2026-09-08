---
id: RR-9MLJMO
type: review-response
title: echoesWired bool read outside mu and carried no reason; Config.FS doc overstated an unenforced guarantee
finding: The boolean was read before mu.Lock in StartWatching (safe today by constructor happens-before, undocumented), the error message could mislead once eviction was the cause, and the Config.FS comment read 'must be the BOTTOM-MOST FS' as if enforced.
severity: nit
resolution: Replaced with echoWiringErr error (documented immutable after New, carrying the specific failure StartWatching returns); Config.FS doc now states the expectation, that it is not type-enforced, and the runtime consequence (ErrWatchNeedsObservableFS).
status: addressed
---
