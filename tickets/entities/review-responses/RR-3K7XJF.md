---
id: RR-3K7XJF
type: review-response
title: pageOutgoingContent re-checks the row face
finding: The per-row face check repeats the query's FromFace filter.
severity: minor
reason: The check is what stops a row from receiving another row's face's edges when the same id appears twice on a page; removing it would make that case wrong rather than visible. It costs one comparison per edge.
status: wont-fix
---
