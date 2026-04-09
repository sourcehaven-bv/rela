---
id: RR-N4FGY
type: review-response
title: CheckContentRule should route through validation, not workspace wrapper
finding: CheckContentRule is already called inside validation.Service. The dataentry and mcp packages duplicated the validation logic instead of using validation.Service. Adding another wrapper on Workspace papers over the real problem. The right fix is to consolidate the duplicated checkValidationRule functions to use validation.Service, eliminating the markdown import without any new wrapper.
severity: significant
resolution: 'Plan updated: will consolidate the duplicated checkValidationRule in dataentry and mcp to call a shared validation function. Since both packages already depend on filter and metamodel, and workspace already wraps validation, the cleanest path is to add a workspace method that delegates to validation.Service for rule checking. The markdown.CheckContentRule call stays in validation (allowed dependency) and disappears from consumers entirely.'
status: addressed
---
