---
finding: 'Interpolated template values are not validated. A user could create an entity with kind: ''../../../etc/passwd'' causing path traversal. Planning doc identified this risk but validation was not implemented.'
id: RR-47km
resolution: Added isValidTemplateName() function in engine.go that rejects path separators (/ \) and double dots (..). Added TestEngine_CreateEntity_TemplatePathTraversal with 6 test cases.
severity: critical
status: addressed
title: Missing template name validation for path traversal
type: review-response
---
