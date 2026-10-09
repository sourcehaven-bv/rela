---
id: RR-FE0FV0
type: review-response
title: Edge clash refused after the destination row was written
finding: An edge clash was detected after CreateEntity so a refused move left a duplicate row on fs/mem.
severity: minor
resolution: Entity and edge clashes are checked before the first write.
status: addressed
---
