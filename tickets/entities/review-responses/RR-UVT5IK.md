---
id: RR-UVT5IK
type: review-response
title: entity_type should not come from the client
finding: The client cannot know the type for documents, forms and entity pages without duplicating config lookups.
severity: significant
resolution: 'Planned: drop entity_type. The server derives the type from config (route type for the entity surface, DocumentConfig/Page/Form entity type) and reads with src.Context. Miss/hidden/wrong type means no candidate; gate infrastructure errors still propagate.'
status: addressed
---
