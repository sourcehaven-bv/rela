---
id: RR-3L3OU7
type: review-response
title: Small scheduler log and bookkeeping nits
finding: already_delivered counts duplicate ids; occurrence re-extracted from the payload map; 'already handled by another scheduler' is also logged on one node; a jobFor error is logged as 'could not create run'; Services.Close never closes the scheduler state; pg Seed with zero times writes touched_at=0001-01-01.
severity: nit
resolution: 'All six fixed: already_delivered is computed from the compacted ids; jobFor returns the occurrence; the ErrRunActive/ErrStale skip says ''task state changed since this tick read it''; a jobFor error logs ''could not build job''; Services.Close closes the scheduler state after the queue drains; Seed with no LastRun and no NextRetry is a no-op on both backends (conformance SeedWithoutTimesIsNoOp).'
status: addressed
---
