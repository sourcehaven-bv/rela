---
id: RR-8GQ5T2
type: review-response
title: 'Seed staleness: reused server frozen at the first screenshot''s seed'
finding: The Capturer stands up the temp-project server once (lazy) with the seedOps as of the FIRST screenshot{}. But seedOps is a build-wide accumulator; a manual with screenshot#1, then a create(), then screenshot#2 would reuse a server seeded with only the first set — the second entity was written to the in-mem store (Tier-A resolvers see it) but never replayed into the temp fsstore, so the SPA renders 'entity not found'. Invisible until a two-figure manual; the single-screenshot example sidesteps it.
severity: critical
resolution: 'Added project.syncSeed(ctx, seed): tracks a high-water mark of applied ops and, on each Capture, replays only the new tail (seed[seeded:]) against the running store. ensure() calls it for the reused-project path; standUp sets seeded=len(seed). Turns the reused-server design into a feature (incremental fixtures across figures). Regression test TestCapture_SeedGrowsAcrossIslands captures a second entity created after standUp.'
status: addressed
---
