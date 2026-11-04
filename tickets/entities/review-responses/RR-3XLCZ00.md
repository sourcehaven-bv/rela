---
id: RR-3XLCZ00
type: review-response
title: RlInlineEdit per cell bypasses the widget registry
finding: frontend/CLAUDE.md requires widgets on every surface; RlInlineEdit gives enum/date/boolean columns a text box. Use defaultRegistry widgets in edit mode.
severity: significant
resolution: Cells render through InlinePropertyValue with the registry widget for the property (defaultRegistry.resolve).
status: addressed
---
