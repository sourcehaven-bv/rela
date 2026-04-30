---
id: RR-JW7GG
type: review-response
title: AdHocFilterMenu silently flips to all-types mode when entityType is briefly undefined
finding: EntityList passes :entity-type=entityType which is EntityType | undefined. While the schema store is loading (or if a list config references a missing entity type), this is undefined. The menu falls into the else branch and renders properties from EVERY entity type. User picks a property like status, applies it, and EntityList writes filter[status]=... but the property may not exist on the lists entity type, producing a confusing zero-result page.
severity: critical
resolution: 'Added required `mode: ''list'' | ''search''` prop to AdHocFilterMenu. In list mode without entityType (schema loading or unknown type) the menu now renders no options instead of falling back to the all-types union. Added regression test in AdHocFilterMenu.test.ts.'
status: addressed
---
