---
id: RR-5KFD7W
type: review-response
title: Planned mixed-render e2e integration test was never written
finding: 'PLAN-6RDYUL''s Test Plan called for ''an e2e case loading a view with mixed `render` values, asserting one field is an input and its neighbour plain text in the same section — the mixed-rendering layout case unit tests cannot cover''. No such spec existed. The only e2e touching this surface was the #997 unmount guard, which had merely been patched to keep passing. The Test Plan box was effectively checked over a missing artifact.'
severity: significant
resolution: 'Added e2e/tests/view-section-render-mode.spec.ts with four specs: (1) a display section and an inline-edit section coexist on one page; (2) a display-default field renders no select/input/FieldShell but still shows its value; (3) a display section reflects the current server value (the RR-GLK4UY staleness guard); (4) an opted-in field renders an enabled control. All four pass. The fixture''s `task` view was left deliberately mixed to support them.'
status: addressed
---

The plan explicitly flagged this case as uncoverable by unit tests, which is
correct: the component tests can prove each arm in isolation, but only a real
page shows that a display section and an inline-edit section coexist without one
stealing the other's chrome.

Full suite after the addition: **235 passed, 0 failed** (was 231), 8 skipped
(postgres-gated history specs, unrelated).
