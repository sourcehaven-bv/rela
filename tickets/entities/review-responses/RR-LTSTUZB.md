---
id: RR-LTSTUZB
type: review-response
title: 'Design: forked run-level instance must fail closed'
finding: The plan keeps the run-level instance at the fork state with a live generation, so every reader not taught about branches (claimable, complete, advance timers, flowedits, header writes, older binaries) treats it as a live step and could fork again.
severity: critical
resolution: 'Plan updated: while forked the run-level instance has State empty and a nonzero generation, so flow.Read reports it stranded and every unaware path refuses it; Previous keeps the fork state. The wire generation of a forked run is 0. Steps are found through one token lookup that never returns the run-level instance while forked.'
status: addressed
---
