---
id: RR-ODSY8
type: review-response
title: Test plan does not actually cover the reported bug
finding: 'The plan proposes a unit test for an entityDetailHref helper, but the bug is not in URL construction — it''s that the click handler passes wrong params and the anchor lacks href. A pure helper test cannot catch a regression where someone refactors and re-passes entity.id, or removes :href from the template. Need a component-level test (Vue Test Utils) that mounts CustomView with stubbed fetchView returning a display: list section, asserts the anchor''s href attribute, and asserts that click triggers router.push with the right path.'
severity: critical
resolution: Test plan now includes (1) Vitest unit test for entityDetailHref helper, (2) Vitest component test on CustomView.vue asserting rendered href + click behavior, (3) e2e test in Playwright. The component test catches the 'someone re-passes entity.id' regression class.
status: addressed
---
