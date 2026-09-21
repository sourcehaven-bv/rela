---
id: RR-6A9A8P
type: review-response
title: The test suite systematically could not observe the pre-response window where the highlight bugs lived
finding: 'Every test in the combined-highlight block did `setQuery(...)` then `await settle()` before asserting, so the search always resolved and `runSearch` always reset the highlight before any assertion ran. All four critical highlight bugs were structurally unreachable by that design, which is why they shipped past the first round of tests. The block was also fragile in the same way as the fixture bugs found during implementation: ''wraps across both sections'' derived `total` from live state, so it would still pass with an empty type section while claiming to test cross-section wrap, and the `typeCount > 0` guard existed in only one test of the block.'
severity: significant
resolution: Added a dedicated 'the highlight survives rows changing underneath it' block whose five tests all assert BEFORE `settle()`, in the window between a keystroke and the response, with a comment saying why. Added 'closed-menu and disposed guards' for the state-leak cases. The stranded-index test was strengthened to highlight the LAST entity after mutation testing showed the first version passed while the bug was present. Two of my own new tests initially failed on fixture setup (entities that did not match the query), the same class of error the reviewer flagged, and were fixed to use matching fixtures.
status: addressed
---
