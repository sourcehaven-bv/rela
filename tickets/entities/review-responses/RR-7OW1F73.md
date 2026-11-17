---
id: RR-7OW1F73
type: review-response
title: Old id generators have no production callers
finding: GenerateNextID, GenerateSequentialID, GenerateShortID and ExtractHighestNumber are unused in production.
severity: nit
reason: Public helpers in internal/entity with their own tests; removal is unrelated cleanup.
status: wont-fix
---
