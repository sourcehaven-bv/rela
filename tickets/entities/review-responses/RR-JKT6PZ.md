---
id: RR-JKT6PZ
type: review-response
title: Prune(keep []string) causes mutual erasure between nodes with different schedules.yaml
finding: 'Prune(keep []string) deletes state for every task not in the caller''s config. On a shared postgres table with two schedulers mid-rollout holding DIFFERENT schedules.yaml, each node erases the other''s tasks on every startup — silently resetting retry ladders and making live tasks re-fire as ''first run''. Today this is harmless only because each node has its own state document; a shared table turns it into a mutual-erasure loop. It also inverts the precedent: userstate.Prune is time-based GC whose doc states ''correctness must not depend on a sweeper the operator might disable'', and it cannot destroy live state no matter what is passed, whereas Prune(keep) can erase everything if handed a partially-loaded config.'
severity: critical
status: open
---
