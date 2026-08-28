---
id: RR-BVQVTT
type: review-response
title: New leaves state nil; recordFailure widens the nil-map panic surface
finding: 'New does not initialise s.state, relying on Run calling loadState first. Pre-existing, but this change widens it: recordFailure now writes two additional maps on the same nil struct. CLAUDE.md requires constructors to reject nil required fields rather than defer the failure. Setting state: newState() in New removes the temporal coupling; TestNew does not assert s.state at all.'
severity: significant
resolution: 'New now sets state: newState(), removing the temporal coupling to loadState. Surfaced a real latent bug while fixing it: pruneOrphanedState panicked on a nil config in an existing test, so it now returns early rather than treating every entry as orphaned, which would have wiped the whole state file.'
status: addressed
---
