---
id: RR-FL9F28
type: review-response
title: cardRelations read the affordance-filtered field list in handleSubmit
finding: handleSubmit built cardRelations from fields.value while the new ownership filter read allFields.value. The two differ exactly when affordances hide a field, so an affordance-hidden card relation was owned but not excluded, letting a stale relations.value entry reach the payload as a full-replace.
severity: significant
resolution: cardRelations now reads allFields.value, matching buildAutoSaveRelationsBody and routePrefilledCardRelations. Whether an edge is card-delivered is a property of the config, not of what is on screen.
status: addressed
---

placeholder
