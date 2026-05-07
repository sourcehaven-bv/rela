---
id: RR-RESXE
type: review-response
title: EntityList migration is not gated on config presence; helper signature is leaky
finding: 'Helper takes schemaStore as parameter -- ties every test to constructing Pinia. Better: take a getDetailView(type) => string|undefined callback, or make it a composable. Also: plan migrates EntityList.vue to use the new helper but doesn''t address that the helper''s chain (cellLink -> entity_types -> /entity) isn''t quite the same as EntityList''s existing chain (column-link -> list.detail_view -> /entity). Document that they''re equivalent post-migration, and ensure tests cover it.'
severity: significant
resolution: 'Helper signature changed: takes a getDetailView(type) => string|undefined callback instead of the schemaStore. Pure function, testable without Pinia. EntityList.vue is now migrated to the helper in the same PR (single source of truth for the priority chain), with column-link passed via opts.cellLink.'
status: addressed
---
