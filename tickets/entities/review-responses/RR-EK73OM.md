---
id: RR-EK73OM
type: review-response
title: Choice buttons depend on the `from` field being rendered
finding: 'Step 9 renders the group "in place of" the list field named by `choices.from`. Fields come from resolveChannelFields with the data-entry presentation, so a presentation that hides or omits that field also removes the buttons. State the intended behaviour (recommend: render the group whenever choiceViews has entries, after the fields) and add a story or test.'
severity: minor
resolution: The choice group renders after the fields whenever there are choice views, also when the presentation hides the from field, and the from field is not rendered twice. AC 12 adds a test or story for it.
status: addressed
---
