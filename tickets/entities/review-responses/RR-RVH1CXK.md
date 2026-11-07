---
id: RR-RVH1CXK
type: review-response
title: E2E page object breaks
finding: faces.page.ts selectWorld uses selectOption and expectSelectedWorld uses toHaveValue; switchToFace looks for button.face-single. The faced fixture has two worlds and two faces, so the segmented control and toggle replace both.
severity: significant
resolution: 'Addressed in plan: the page object is rewritten to click the link by label and to assert aria-current. The faces specs run in the test step.'
status: addressed
---
