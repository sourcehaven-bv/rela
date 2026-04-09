---
id: RR-REYDV
type: review-response
title: Duplicated checkValidationRule is root cause of CheckContentRule leak
finding: dataentry/analyze.go and mcp/tools_helpers.go have nearly identical checkValidationRule methods that duplicate validation.Service.checkEntityAgainstRule. The plan explicitly acknowledges this but refuses to consolidate. However, consolidation would eliminate the markdown.CheckContentRule calls from both consumers automatically — no wrapper needed.
severity: significant
resolution: 'Merged with finding #4. Plan now consolidates the duplicated validation logic as the primary strategy instead of adding wrappers. Both consumers will call a workspace method that delegates to validation internals.'
status: addressed
---
