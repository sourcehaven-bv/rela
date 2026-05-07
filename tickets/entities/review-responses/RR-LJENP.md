---
id: RR-LJENP
type: review-response
title: cell.entityId || row.entityId may navigate to wrong entity for relation columns
finding: Relation-column cells show *related* entities (sections.go:220), but cell.EntityID is set to the row entity. So clicking a relation cell navigates to the row entity, not the related one. Pre-existing latent issue but the plan should explicitly accept it (current behavior, stable) or scope a separate fix — don't try to fix it as part of this bug because the backend doesn't expose related-entity types per cell.
severity: minor
reason: Pre-existing relation-column cell navigation issue (clicks on cells showing related entities navigate to row entity, not related). Out of scope for this bug. Plan adds a code comment flagging it; separate ticket can address it once backend exposes related-entity types per cell.
status: deferred
---
