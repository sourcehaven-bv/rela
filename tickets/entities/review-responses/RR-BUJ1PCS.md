---
id: RR-BUJ1PCS
type: review-response
title: MCP property analysis has the same bug
finding: internal/mcp/tools_analysis.go runs ValidateEntity over the gated store, so a restricted MCP caller also gets findings on hidden fields, without a field name.
severity: significant
resolution: Both surfaces now call metamodel.Validator.ValidateReadable, which drops hidden-field findings and names the field. TestAnalyzeProperties_SkipsHiddenFields covers MCP.
status: addressed
---
