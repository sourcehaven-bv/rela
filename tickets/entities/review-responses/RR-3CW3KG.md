---
id: RR-3CW3KG
type: review-response
title: DynamicForm exclusion rests on the wrong argument; the real reason is it has no banner surface
finding: 'The scope decision was justified by notEditable being `!actionAllowed(entity, ''update'')`, so a writable draft never reaches that branch. True, but incomplete: the edit form has NO banner surface at all — there is not one WorldBanner or world-banner in DynamicForm.vue. notEditableNote is the body text of a refusal screen, not chrome. So adding a notice there would be building a new surface, not extending this feature. There IS a real gap (an operator editing a draft sees no ''not yet in force'' reminder on the form where they are changing it) but closing it is a separate ticket with a design question. Recorded so a reviewer checking only the notEditable argument does not conclude the form was covered.'
severity: minor
resolution: 'Added a ''Not done: the edit form'' section to the ticket body recording the stronger reason: DynamicForm has NO banner surface at all (not one WorldBanner in the file), and notEditableNote is the body text of a refusal screen rather than chrome — so a notice there means designing a new surface, not extending this feature. The section also names the real gap left open (an operator editing a draft sees no not-yet-in-force reminder on the form where they are changing it) and says it is a separate ticket with a design question, so a future reader cannot mistake the exclusion for coverage.'
status: addressed
---
