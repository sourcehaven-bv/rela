---
id: RR-86EDV4
type: review-response
title: 300-character limit needs a named shared constant and a stated basis
finding: '300 appears in realtimecfg and again as maxLength in OpenAPI (plan line 367): two literals that drift. Put the constant beside buttonRunes (internal/realtimecfg/choices.go:9-10) or in realtimelimits, and have internal/openapi read it, as other limits are quoted from realtimelimits. Count runes after foldDisplay, as the label does (buttons.go:84-90), and say so: JSON Schema maxLength counts code points, so the two agree only if both count runes. State the basis in the plan; Slack Block Kit''s confirm text, which the plan cites as the closest match, is capped at 300 characters.'
severity: minor
resolution: Added realtimelimits.MaxConfirmTextRunes = 300, counted in runes after foldDisplay, and OpenAPI quotes it as maxLength. The basis is Slack Block Kit confirm text, capped at 300.
status: addressed
---
