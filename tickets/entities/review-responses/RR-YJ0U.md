---
id: RR-YJ0U
type: review-response
title: TaskCheckBox is inline child of TextBlock
finding: TaskCheckBox is an inline node inside the TextBlock, not a direct child of ListItem. Implementation must walk into TextBlock children to detect it (or check first inline of TextBlock).
severity: nit
resolution: Documented in plan's technical approach with reference to internal/markdown/content.go pattern.
status: addressed
---
