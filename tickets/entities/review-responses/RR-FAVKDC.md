---
id: RR-FAVKDC
type: review-response
title: No restoreAllMocks between tests (spy hygiene)
finding: Each behavioural test called `vi.spyOn(router, 'push')` after mounting, with no `vi.restoreAllMocks()` in teardown. Re-spying an already-spied method can return the existing spy, so accumulated call counts could mask a double-registration rather than reveal it.
severity: minor
resolution: '`vi.restoreAllMocks()` added to the `afterEach` in both behavioural test files, alongside the wrapper unmount from RR-SNHEFQ (one block, as the reviewer suggested). The new call-count assertions depend on this being clean, so the two fixes reinforce each other.'
status: addressed
---

Finding 6 from the cranky-code-reviewer.
