---
id: RR-QWANO0
type: review-response
title: 'Composer: single transition type and live reorder'
finding: 'The segmented control renders only when offered.length > 1 (RlChannelComposer.vue:320-327). With one offered type that is a transition, the workflow icon and srHint never render and only the foot hint marks it; no story covers that case. A move by another user also reorders the radio options while the user may have focus in the group. Plan change: add a story OneTransitionType (the hint shows and is referenced by the submit button). Say in the plan that the selected value and focus stay on the same option when transitions change. Add a play test that changes transitions while an option has focus and checks that document.activeElement is still that option.'
severity: minor
resolution: 'Plan updated: AC 12 adds OneTransitionType (hint shown, referenced by submit) and ReorderKeepsFocus (selection and document.activeElement kept); Approach 6 states selection and focus stay on the same option.'
status: addressed
---
