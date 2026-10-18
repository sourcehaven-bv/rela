---
id: RR-L3MZ4Y
type: review-response
title: Exhaustive clamp test misses except ceilings and read faces
finding: TestClamp_IsExactIntersection builds no ceiling with except (scope reopen under deny) and checks only create via GrantsVerbOnState, not read face semantics.
severity: minor
resolution: Added a deny [*] / except [page] ceiling and a read check through roleGrantsRead and roleReadFaces.
status: addressed
---
