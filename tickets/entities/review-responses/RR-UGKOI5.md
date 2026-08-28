---
id: RR-UGKOI5
type: review-response
title: No global export concurrency limit in v1
finding: Many simultaneous exports each spawn a subprocess + temp dir. Per-call timeout and output cap exist, but there is no global concurrency limit, so a burst could spawn many converters at once.
severity: minor
resolution: 'Implemented in v1 after all: transform.Engine wraps cmdexec with a bounded worker pool (WithMaxConcurrent, default 4) shared across requests — exports past the bound queue rather than pile on. The engine is constructed once per handler (pinned by TestExport_EngineIsSharedAcrossRequests), so the cap is a real global bound. Landed with the cmdexec confinement work and the engine-lifecycle simplification in PR #1188.'
reason: v1 targets fast converters (pandoc) and the per-call timeout+cap bound each invocation; a global semaphore is a straightforward v2 addition alongside the async job model for slow converters. Documented as a known bound in the plan risks.
status: addressed
---
