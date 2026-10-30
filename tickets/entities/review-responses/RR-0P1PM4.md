---
id: RR-0P1PM4
type: review-response
title: Line-break rationale contradicts ConfirmModal; texthazard already refuses it
finding: 'The plan refuses a line break because "the dialog shows one paragraph" (plan line 304), but ConfirmModal renders its description with white-space: pre-line (frontend/src/components/ui/ConfirmModal.vue:102-104), so breaks would show. texthazard.Find already treats a newline as a hazard (internal/texthazard test case "newline" true), so the line-break bullet in AC 2 is only a friendlier message for the same refusal. Keep the refusal for consistency with the label rules, give the true reason, and check the line break before texthazard so the operator gets the specific message. Note that foldDisplay trims, so the trailing newline of a | block scalar is not a break.'
severity: minor
resolution: The line-break refusal is kept for consistency with the label rules, checked before texthazard for a specific message, with the ConfirmModal pre-line fact stated. The plan notes that foldDisplay trims the trailing newline of a | block.
status: addressed
---
