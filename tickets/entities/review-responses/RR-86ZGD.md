---
id: RR-86ZGD
type: review-response
title: Six copies of stringValue computed
finding: Text/Textarea/Number/Date/Select/Rrule all carry identical stringValue with null/undefined guard. Six copies, past the CLAUDE.md three-line threshold. Zero behavioural variance. A 5-line composable is a clean extraction with no new boundary.
severity: minor
resolution: Extracted frontend/src/widgets/useStringValue.ts (a 5-line composable). Six widgets (Text, Textarea, Number, Date, Select, Rrule) now call useStringValue(() => props.modelValue) instead of duplicating the inline computed. Zero behavioural change; ~30 lines of duplication gone.
status: addressed
---
