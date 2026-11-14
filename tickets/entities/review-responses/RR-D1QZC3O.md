---
id: RR-D1QZC3O
type: review-response
title: Combined PATCH of properties and relations judged on stale edges
finding: Data-entry PATCH stores the entity patch before the relation writes. A PATCH setting status in_behandeling and adding the plan was refused; a PATCH changing a property and removing the last plan stored the property and then got a 422.
severity: significant
resolution: The PATCH plans the ruled edge changes before the entity patch and passes them with entitymanager.WithPendingRelations. The update check judges the new state against the edges the PATCH leaves. Pinned by TestRelationRule_PatchJudgesTheStateItLeaves.
status: addressed
---
