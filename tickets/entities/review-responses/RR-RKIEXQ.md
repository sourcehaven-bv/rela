---
id: RR-RKIEXQ
type: review-response
title: SearchVisibleFields doc described the old single-pass cost model
finding: The godoc still said the entity is in hand from the visibility scan; bodies are now fetched in a second statement.
severity: significant
resolution: Doc rewritten to describe the two passes and the invariant that pass 1 can only over-approximate the rows needing a body.
status: addressed
---
