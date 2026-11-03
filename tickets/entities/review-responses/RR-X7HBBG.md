---
id: RR-X7HBBG
type: review-response
title: Every misplaced compose decision_button.confirm warning would say the presets set their own confirm
finding: 'In `effect`''s no-type-button branch, the plan counts a compose preset as setting its own `confirm` (Approach 5). The warning then reads "every entry of X has a preset button that sets its own confirm" (internal/realtimecfg/buttonoptions.go:205-207), which is false for a compose button. Give the case its own text, such as "confirm has no effect: every entry of X has a preset button that sets its own confirm or opens the composer". Assert the text in TestButtonOptions_Compose.'
severity: minor
resolution: ADDRESSED The warning reads "...sets its own confirm or opens the composer" when a compose preset is among them, and the test asserts the full text (AC 5, Approach 5).
status: addressed
---
