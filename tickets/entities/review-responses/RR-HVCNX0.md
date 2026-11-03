---
id: RR-HVCNX0
type: review-response
title: Clicked button gets native disabled plus loading, so focus drops to body
finding: 'Approach 2 sets `disabled` on every button while `decisionsDisabled` is true, and `loading` on the clicked one. RlButton.vue (lines ~124-135) uses aria-disabled while loading ''because native disabled drops focus to <body>'', and ''never both'': once disabled is set, native disabled wins. During the in-flight request, and after an error re-enables the buttons, a keyboard or screen reader user has lost their place. frontend/CLAUDE.md ''Pending and loading indicators / Accessibility'' states the same rule. Pass disabled only to the buttons that are not pending, and make AC 8''s story assert that the pending button keeps focus and is aria-disabled, not disabled.'
severity: minor
resolution: The pending button gets only loading (aria-disabled, keeps focus) and the other buttons get disabled, asserted in the RlChannelDecisions story play.
status: addressed
---
