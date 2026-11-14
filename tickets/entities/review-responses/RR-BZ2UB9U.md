---
id: RR-BZ2UB9U
type: review-response
title: Present and Exit drop the query and the world
finding: Entering or leaving present mode pushed a bare path, losing world and other query keys, so the room saw different data.
severity: significant
resolution: setPresenting pushes the same path with only present changed; step links keep world. Unit tests pin both.
status: addressed
---
