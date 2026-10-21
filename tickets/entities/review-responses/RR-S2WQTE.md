---
id: RR-S2WQTE
type: review-response
title: OpenAPI omits warnings and the 500 conversion_incomplete response
finding: addTaskItemConversionPaths does not document warnings or the kept-entity 500.
severity: nit
resolution: The OpenAPI convert path now documents the 500 conversion_incomplete response, and the Error schema documents entity_id. warnings was already in the result schema. Goldens regenerated.
status: addressed
---
