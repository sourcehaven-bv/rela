---
id: RR-4FD6L2
type: review-response
title: The archplan job had a cold build cache and no timeout
finding: The job is skipped on push, so a cache it saved was scoped to one PR ref. It exports twice and had the default 360-minute timeout.
severity: significant
resolution: The job restores (read-only) the Lint job's build cache, which push builds save, and has a 45-minute timeout. A develop model artifact for --base-model is left for later if the job is still slow.
status: addressed
---
