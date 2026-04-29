---
id: RR-VZA4X
type: review-response
title: executeAction has too many responsibilities
finding: 'Inherited issue: executeAction now runs the action, counts results, dispatches the dialog, shows a toast, clears selection, optimistically updates entities, and schedules a refetch. Future refactor: extract dispatchScriptError and summarizeAndToast. Not blocking.'
severity: nit
resolution: Inherited god-function. Refactoring executeAction's six responsibilities is scope creep for a one-line wiring fix. Track as a follow-up cleanup ticket if/when this composable next needs substantive change.
reason: Inherited god-function. Refactoring executeAction's six responsibilities is scope creep for a one-line wiring fix. Track as a follow-up cleanup ticket if/when this composable next needs substantive change.
status: deferred
---
