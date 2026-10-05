---
id: RR-TQFFLF
type: review-response
title: Importer skipped face validation for sources only in the import batch
finding: Batch-only entities were recorded with type empty, so requireTail returned early and a content edge from a batch-only faced source with an undeclared face passed validation and was written straight to the store.
severity: significant
resolution: knownEntities records the batch row's validated type; the tail check and ValidateRelation now run for batch-only endpoints. Test case 'undeclared tail on a source only in the batch' in TestImport_FaceValidation.
status: addressed
---
